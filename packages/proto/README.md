# @agogo/proto

Shared ABI contracts for the editor shell and the Go/Wasm engine.

## UI primitive split

This repository uses a split between shadcn-style wrappers and Base UI primitives:

- shadcn-style wrappers: `Button`, `Dialog`, `Separator`, `ScrollArea`, `Tooltip`, `DropdownMenu`
- Base UI primitives: `Menu`, `Popover`, `Listbox`, `Slider`

The shell starts with shadcn-compatible local components so the app can be scaffolded before the full Base UI integration is wired up. The Base UI primitives are reserved for more interactive controls where a headless, composable primitive is a better fit.

## Phase notes

- `src/commands.ts` holds command IDs shared by the frontend and engine.
- `src/responses.ts` holds the render result shape returned from Wasm.
