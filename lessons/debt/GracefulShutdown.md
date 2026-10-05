Graceful shutdown
Containers get SIGTERM and then, some seconds later, SIGKILL. Between those you want in-flight requests to finish:

func run() error {
    ctx, stop := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer stop()

    srv := &http.Server{ /* as above */ }

    errCh := make(chan error, 1)
    go func() {
        slog.Info("listening", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil &&
            !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        slog.Info("shutting down")
    }

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return srv.Shutdown(shutdownCtx)
}