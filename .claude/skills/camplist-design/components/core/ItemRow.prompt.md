One line of a packing list — name, category, actions — with a hairline divider; put rows inside a Card.

```jsx
<Card>
  <ItemRow><span>Tent</span><span className="muted">Shelter</span></ItemRow>
  <ItemRow last><span>Sleeping bag</span><span className="muted">Sleep</span></ItemRow>
</Card>
```

Pass `last` on the final row to drop its divider. Push actions right with `style={{marginLeft:'auto'}}` on the action.
