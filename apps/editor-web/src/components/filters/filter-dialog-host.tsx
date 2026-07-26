import { FadeDialog } from "@/components/filters/fade-dialog";
import { FilterDialog } from "@/components/filters/filter-dialog";
import { useFilterState } from "@/state/filter-state";
import { useEngine } from "@/wasm/context";

/**
 * Mounts the active filter dialog and the fade dialog, wiring them to the
 * shared filter state and the engine. The filter dialog is keyed by filter id
 * so switching filters re-seeds the parameter drafts from a clean mount.
 */
export function FilterDialogHost() {
  const engine = useEngine();
  const {
    activeFilterId,
    fadeOpen,
    lastFilter,
    closeFilter,
    closeFade,
    noteFilterApplied,
    noteFaded,
  } = useFilterState();

  return (
    <>
      {activeFilterId ? (
        <FilterDialog
          key={activeFilterId}
          filterId={activeFilterId}
          engine={engine}
          onClose={closeFilter}
          onApplied={noteFilterApplied}
        />
      ) : null}
      {fadeOpen ? (
        <FadeDialog
          engine={engine}
          filterName={lastFilter?.name ?? "Filter"}
          onClose={closeFade}
          onFaded={noteFaded}
        />
      ) : null}
    </>
  );
}
