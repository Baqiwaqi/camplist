package validation

import (
	"camplist/internal/packing"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// External validation is opt-in. Emulator mode never reads .env.
func validationStore(t *testing.T, owner string) *packing.Store {
	if os.Getenv("CAMPLIST_LIVE_COSMOS") == "1" && os.Getenv("CAMPLIST_EMULATOR") == "1" {
		t.Fatal("choose live Cosmos or emulator mode, not both")
	}

	if os.Getenv("CAMPLIST_LIVE_COSMOS") == "1" {
		return liveStore(t, owner)
	}
	t.Helper()
	if os.Getenv("CAMPLIST_EMULATOR") != "1" {
		t.Skip("set CAMPLIST_EMULATOR=1 to validate the local Cosmos emulator")
	}
	cred, err := azcosmos.NewKeyCredential(base64.StdEncoding.EncodeToString(make([]byte, 64)))
	if err != nil {
		t.Fatal(err)
	}
	client, err := azcosmos.NewClientWithKey("http://127.0.0.1:8081", cred, &azcosmos.ClientOptions{ClientOptions: policy.ClientOptions{InsecureAllowCredentialWithHTTP: true}, EnableContentResponseOnWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("camplist-validation-%d", time.Now().UnixNano())
	if _, err = client.CreateDatabase(ctx, azcosmos.DatabaseProperties{ID: name}, nil); err != nil {
		t.Fatal(err)
	}
	database, err := client.NewDatabase(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := database.Delete(ctx, nil); err != nil {
			t.Errorf("clean up validation database: %v", err)
		}
	})
	_, err = database.CreateContainer(ctx, azcosmos.ContainerProperties{ID: "packing", PartitionKeyDefinition: azcosmos.PartitionKeyDefinition{Paths: []string{"/userId"}, Kind: azcosmos.PartitionKeyKindHash}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	container, err := client.NewContainer(name, "packing")
	if err != nil {
		t.Fatal(err)
	}
	return packing.NewStore(container)
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func TestCosmosValidation(t *testing.T) {
	owner := fmt.Sprintf("camplist-validation-%d", time.Now().UnixNano())
	store := validationStore(t, owner)
	ctx := context.Background()
	list := packing.NewList(owner, "Validation weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen")}
	must(t, store.SavePackingList(ctx, list))
	other := packing.NewList(owner+"-other", "Private", "")
	other.ID = list.ID
	must(t, store.SavePackingList(ctx, other))
	got, err := store.GetPackingList(ctx, list.ID, owner+"-other")
	must(t, err)
	if got.Name != "Private" {
		t.Fatal("partition isolation failed")
	}
	added := packing.NewItem("Matches", "Kitchen")
	must(t, store.AddItem(ctx, list.ID, owner, added))
	must(t, store.RemoveItem(ctx, list.ID, owner, added.ID))
	session, err := store.CreatePackingSession(ctx, list.ID, owner)
	must(t, err)
	lists, err := store.GetPackingLists(ctx, owner)
	must(t, err)
	if len(lists) != 1 {
		t.Fatal("list query returned mixed types")
	}
	sessions, err := store.ListPackingSession(ctx, owner)
	must(t, err)
	if len(sessions) != 1 {
		t.Fatal("session query returned mixed types")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, item := range list.Items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.SyncSessionItem(ctx, session.ID, owner, packing.PackingOperation{ID: fmt.Sprint("parallel-", i), ItemID: item.ID, Checked: true})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		must(t, err)
	}
	current, err := store.GetPackingSession(ctx, session.ID, owner)
	must(t, err)
	if current.List.CountChecked() != 2 {
		t.Fatal("concurrent different-item writes lost data")
	}
	op := packing.PackingOperation{ID: "parallel-0", ItemID: list.Items[0].ID, Checked: true}
	_, err = store.SyncSessionItem(ctx, session.ID, owner, op)
	must(t, err)
	current, err = store.GetPackingSession(ctx, session.ID, owner)
	must(t, err)
	if current.List.Items[0].Revision != 1 {
		t.Fatal("duplicate acknowledgement advanced revision")
	}
	op.ID = "stale"
	op.Checked = false
	_, err = store.SyncSessionItem(ctx, session.ID, owner, op)
	if !errors.Is(err, packing.ErrConflict) {
		t.Fatalf("missing revision conflict: %v", err)
	}
	_, err = store.GetPackingSession(ctx, session.ID, owner+"-other")
	var response *azcore.ResponseError
	if !errors.As(err, &response) || response.StatusCode != 404 {
		t.Fatalf("cross-account session read: %v", err)
	}
	template, err := store.GetPackingList(ctx, list.ID, owner)
	must(t, err)
	entry := packing.ReviewEntry{ID: "fuel", Name: "Stove", NeedsAttention: true, Action: packing.ReviewTask, Task: "Buy fuel"}
	_, err = store.AddReviewEntry(ctx, session.ID, owner, entry)
	must(t, err)
	updated, err := store.ApplyReview(ctx, session.ID, owner, template.Revision(), []string{entry.ID})
	must(t, err)
	_, err = store.ApplyReview(ctx, session.ID, owner, template.Revision(), []string{entry.ID})
	must(t, err)
	if len(updated.Tasks) != 1 {
		t.Fatal("review task missing")
	}
	stale := template
	stale.Name = "Stale edit"
	err = store.SavePackingList(ctx, stale)
	if !errors.As(err, &response) || response.StatusCode != 412 {
		t.Fatalf("stale template replaced newer review: %v", err)
	}
	next, err := store.CreatePackingSession(ctx, list.ID, owner)
	must(t, err)
	if len(next.Improvements) != 1 || len(next.List.Tasks) != 1 {
		t.Fatal("next-trip improvement missing")
	}

	// Two different operations against the same starting revision cannot both win.
	outcomes := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := store.SyncSessionItem(ctx, next.ID, owner, packing.PackingOperation{ID: fmt.Sprint("race-", i), ItemID: next.List.Items[0].ID, Checked: i == 0})
			outcomes <- err
		}()
	}
	wins, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-outcomes
		if err == nil {
			wins++
		} else if errors.Is(err, packing.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("same-item race: %d wins, %d conflicts", wins, conflicts)
	}
	must(t, store.DeletePackingList(ctx, list.ID, owner))
	lists, err = store.GetPackingLists(ctx, owner)
	must(t, err)
	if len(lists) != 0 {
		t.Fatal("soft-deleted list remains visible")
	}
	_, err = store.GetPackingSession(ctx, session.ID, owner)
	must(t, err)
	_, err = store.RecoverReviewTemplate(ctx, session.ID, owner)
	must(t, err)
	must(t, store.DeletePackingSession(ctx, session.ID, owner))
	_, err = store.SyncSessionItem(ctx, session.ID, owner, op)
	if !errors.As(err, &response) || response.StatusCode != 404 {
		t.Fatalf("deleted session sync: %v", err)
	}
	t.Log("PASS: partitions, queries, patches, concurrent ETags, receipts, conflicts, reviews, recovery and deletion")
}

// Live validation uses only two unique test partitions in the existing container.
// It never creates a database, container, throughput setting or other Azure resource.
func liveStore(t *testing.T, owner string) *packing.Store {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	env, err := godotenv.Read(filepath.Join(filepath.Dir(file), "../..", ".env"))
	must(t, err)
	cred, err := azcosmos.NewKeyCredential(env["DB_KEY"])
	must(t, err)
	client, err := azcosmos.NewClientWithKey(env["DB_URL"], cred, &azcosmos.ClientOptions{EnableContentResponseOnWrite: true})
	must(t, err)
	container, err := client.NewContainer("dev", "packing_list")
	must(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		for _, user := range []string{owner, owner + "-other"} {
			pk := azcosmos.NewPartitionKeyString(user)
			pager := container.NewQueryItemsPager("SELECT c.id FROM c WHERE c.userId = @owner", pk, &azcosmos.QueryOptions{QueryParameters: []azcosmos.QueryParameter{{Name: "@owner", Value: user}}})
			var ids []string
			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					t.Errorf("cleanup query failed for %s: %v", user, err)
					break
				}
				for _, data := range page.Items {
					var doc struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(data, &doc); err != nil {
						t.Error(err)
						continue
					}
					ids = append(ids, doc.ID)
				}
			}
			for _, id := range ids {
				if _, err := container.DeleteItem(ctx, pk, id, nil); err != nil {
					t.Errorf("cleanup test document %s: %v", id, err)
				}
			}
			t.Logf("Removed %d validation documents from %s", len(ids), user)
		}
	})
	return packing.NewStore(container)
}
