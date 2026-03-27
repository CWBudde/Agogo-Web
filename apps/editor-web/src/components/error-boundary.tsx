import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "@/components/ui/button";

type ErrorBoundaryProps = {
  children: ReactNode;
};

type ErrorBoundaryState = {
  error: Error | null;
};

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Editor shell error boundary caught:", error, info);
  }

  override render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-background p-6 text-foreground">
          <div className="editor-surface max-w-xl rounded-[1.5rem] p-6">
            <p className="text-xs uppercase tracking-[0.3em] text-slate-500">Shell Error</p>
            <h1 className="mt-3 text-2xl font-semibold text-slate-100">The editor shell crashed</h1>
            <p className="mt-3 text-sm leading-6 text-slate-300">{this.state.error.message}</p>
            <div className="mt-6">
              <Button
                onClick={() => {
                  this.setState({ error: null });
                  window.location.reload();
                }}
              >
                Reload
              </Button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
