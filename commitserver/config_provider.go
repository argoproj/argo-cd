package commitserver

import (
	"github.com/argoproj/argo-cd/v3/util/configbus"
)

// InitConfigProvider wires the configbus provider after Config is retained.
func (a *ArgoCDCommitServer) InitConfigProvider(crd configbus.CRDSource) {
	a.configProvider = configbus.NewChainProvider(
		configbus.NewCRDProvider(crd),
		&configbus.StaticProvider{Fields: configbus.StaticFields{
			CommitserverGrpcEnableTxtServiceConfig: configbus.Ptr(a.Config.GrpcEnableTxtServiceConfig),
			CommitserverListenAddress:              configbus.Ptr(a.Config.ListenHost),
			CommitserverListenPort:                 configbus.Ptr(a.Config.ListenPort),
			CommitserverLogFormat:                  configbus.Ptr(a.Config.LogFormat),
			CommitserverLogLevel:                   configbus.Ptr(a.Config.LogLevel),
			CommitserverMetricsListenAddress:       configbus.Ptr(a.Config.MetricsHost),
			CommitserverMetricsPort:                configbus.Ptr(a.Config.MetricsPort),
		}},
		configbus.NewEnvProvider(),
	)
}
