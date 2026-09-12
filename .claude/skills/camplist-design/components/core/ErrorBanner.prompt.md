Red validation box shown above form actions when a submit fails — solid red block, white glyph, display heading, messages beneath.

```jsx
<ErrorBanner errors={['Name is required']} />
<ErrorBanner title="Couldn't save" errors={['Storing packing list failed']} />
```

Messages are short sentences, sentence case, no trailing period. Only semantic color in the product; never use it for warnings or info.
