// Package shutdown allows you to gracefully shutdown your app.
package shutdown

// const defaultGraceDuration = 10 * time.Second

// // Hook is a shutdown hook that will be called when signal is received.
// type Hook struct {
// 	Name       string
// 	ShutdownFn func(ctx context.Context) error
// }

// // Shutdown provides a way to listen for signals and handle shutdown of an application gracefully.
// type Shutdown struct {
// 	hooks         []Hook
// 	mutex         *sync.Mutex
// 	logger        *slog.Logger
// 	graceDuration time.Duration
// }

// // Option is the options type to configure Shutdown.
// type Option func(*Shutdown)

// // New returns a new Shutdown instance with the provided options.
// func New(logger *slog.Logger, opts ...Option) *Shutdown {
// 	shutdown := &Shutdown{
// 		hooks:         []Hook{},
// 		mutex:         &sync.Mutex{},
// 		logger:        logger,
// 		graceDuration: defaultGraceDuration,
// 	}

// 	for _, opt := range opts {
// 		opt(shutdown)
// 	}

// 	return shutdown
// }

// // WithHooks adds the hooks to be run as part of the graceful shutdown.
// func WithHooks(hooks []Hook) Option {
// 	return func(shutdown *Shutdown) {
// 		for _, h := range hooks {
// 			shutdown.Add(h.Name, h.ShutdownFn)
// 		}
// 	}
// }

// // WithGraceDuration sets the grace period for all shutdown hooks to finish running.
// // If not used, the default grace period is 10s.
// func WithGraceDuration(gracePeriodDuration time.Duration) Option {
// 	return func(shutdown *Shutdown) {
// 		shutdown.graceDuration = gracePeriodDuration
// 	}
// }

// // Add adds a shutdown hook to be run when the signal is received.
// func (s *Shutdown) Add(name string, fn func(ctx context.Context) error) {
// 	s.mutex.Lock()
// 	defer s.mutex.Unlock()
// 	s.hooks = append(s.hooks, Hook{
// 		Name:       name,
// 		ShutdownFn: fn,
// 	})
// }

// // Hooks returns a copy of the shutdown hooks.
// func (s *Shutdown) Hooks() []Hook {
// 	s.mutex.Lock()
// 	defer s.mutex.Unlock()

// 	hooks := make([]Hook, len(s.hooks))
// 	copy(hooks, s.hooks)

// 	return hooks
// }

// // Listen waits for the signals provided and executes each shutdown hook sequentially in FILO order.
// // It will immediately stop and return once the grace period has passed.
// func (s *Shutdown) Listen(ctx context.Context, signals ...os.Signal) map[string]error {
// 	signalCtx, stopSignalCtx := signal.NotifyContext(ctx, signals...)
// 	defer stopSignalCtx()

// 	<-signalCtx.Done()

// 	start := time.Now()

// 	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.graceDuration)
// 	defer shutdownCancel()

// 	sErr := make(map[string]error, len(s.Hooks()))

// 	hooks := s.Hooks()

// loop:
// 	for i := range hooks {
// 		hook := hooks[len(hooks)-1-i]

// 		s.logger.Info("Shutting down hook", slog.String("hook", hook.Name))

// 		errChan := make(chan error, 1)

// 		// To check the context timeout, so we run shutdown func in goroutine.
// 		// But it still waits for getting the result from errChan before execute the next one.
// 		go func(h Hook) {
// 			errChan <- h.ShutdownFn(shutdownCtx)
// 		}(hook)

// 		select {
// 		case <-shutdownCtx.Done():
// 			// Record current did not shutdown hook
// 			remain := len(hooks) - 1 - i
// 			for idx := remain; idx >= 0; idx-- {
// 				sErr[hooks[idx].Name] = fmt.Errorf("%s did not shutdown within grace period of %v: %w",
// 					hooks[idx].Name, s.graceDuration, shutdownCtx.Err())
// 			}

// 			break loop
// 		case err := <-errChan:
// 			if err != nil {
// 				sErr[hook.Name] = err
// 			}
// 		}
// 	}

// 	s.logger.Info("Time taken for shutdown", slog.String("duration", time.Since(start).String()))

// 	if len(sErr) > 0 {
// 		return sErr
// 	}

// 	return nil
// }
