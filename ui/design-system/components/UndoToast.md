# UndoToast

`src/components/ui/UndoToast.astro` confirms a destructive admin action and gives a 6 second window to undo it.

```astro
<UndoToast message="Post deleted" />
```

**Props:** `message`, what happened in the past tense. The Undo button carries `data-undo` for the script that restores the item in place.
- Club green with light text and `rounded-md` corners; the 2px turf bar drains over 6 seconds. When it runs out the change commits silently.
- `role="status"` announces it; Undo is reachable by keyboard.
- One toast at a time, anchored bottom-left of the admin page.
