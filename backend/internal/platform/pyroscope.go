package platform

import (
	pyroscopego "github.com/grafana/pyroscope-go"

	"github.com/your-org/go-react-starter/backend/internal/config"
)

// NewPyroscope starts continuous profiling against a Pyroscope server.
// If PYROSCOPE_URL is empty it is a no-op, so the binary works without Pyroscope.
func NewPyroscope(cfg config.Config) (func(), error) {
	if cfg.Pyroscope.URL == "" {
		return func() {}, nil
	}
	profiler, err := pyroscopego.Start(pyroscopego.Config{
		ApplicationName: cfg.OTEL.ServiceName,
		ServerAddress:   cfg.Pyroscope.URL,
		Tags:            map[string]string{"env": string(cfg.AppEnv)},
		ProfileTypes: []pyroscopego.ProfileType{
			pyroscopego.ProfileCPU,
			pyroscopego.ProfileMutexCount,
			pyroscopego.ProfileMutexDuration,
			pyroscopego.ProfileAllocObjects,
			pyroscopego.ProfileAllocSpace,
			pyroscopego.ProfileInuseObjects,
			pyroscopego.ProfileInuseSpace,
			pyroscopego.ProfileGoroutines,
			pyroscopego.ProfileBlockCount,
			pyroscopego.ProfileBlockDuration,
		},
	})
	if err != nil {
		return nil, err
	}
	return func() { _ = profiler.Stop() }, nil
}

