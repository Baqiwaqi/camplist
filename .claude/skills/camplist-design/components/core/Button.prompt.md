Pill button. Pine `primary` for creating/saving; ember `accent` for the single "go" action on a page (Start session, Let's pack); secondary for Edit/Details; danger for Delete.

```jsx
<Button>Create New</Button>
<Button variant="accent" size="lg">Start session</Button>
<Button variant="secondary">Edit</Button>
<Button variant="danger" size="sm">Delete</Button>
<Button variant="link">Cancel</Button>
```

Hover darkens one step and lifts 1px (120ms). Never two accent buttons on one screen.
