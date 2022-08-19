package otel

import (
	"context"
	"sync"

	"github.com/paypal/hera/utility/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
)

//"init", "acpt", "wait", "busy", "schd", "fnsh", "quce", "asgn", "idle", "bklg", "strd", "cls"}
//Following Metric Names will get instrumented as part of StateLogMetrics

const (
	// Worker States
	INIT_CONN_COUNT_METRIC      = "hera.init_connection.count"
	ACCPT_CONN_COUNT_METRIC     = "hera.accept_connection.count"
	WAIT_CONN_COUNT_METRIC      = "hera.wait_connection.count"
	BUSY_CONN_COUNT_METRIC      = "hera.busy_connection.count"
	SCHEDULED_CONN_COUNT_METRIC = "hera.schd.connection.count"
	FINISHED_CONN_COUNT_METRIC  = "hera.fnsh.connection.count"
	QUIESCED_CONN_COUNT_METRIC  = "hera.quce.connection.count"
	// Connection States
	ASSIGNED_CONN_COUNT_METRIC = "hera.asgn.connection.count"
	IDLE_CONN_COUNT_METRIC     = "hera.idle_connection.count"
	BACKLOG_CONN_COUNT_METRIC  = "hera.backlog_connection.count"
	STRD_CONN_COUNT_METRIC     = "hera.strd_connection.count"
)

// Object represents the workers states data for worker belongs to specific shardId and workperType with flat-map
// between statename vs count.
type WorkersStateData struct {
	ShardId    int
	WorkerType int
	InstanceId int
	StateData  map[string]int64
}

// state_log_metrics reports workers states
type StateLogMetrics struct {

	//Statelog metrics configuration data
	metricsConfig stateLogMetricsConfig

	meter metric.Meter

	//Channel to receive statelog data
	mStateDataChan <-chan WorkersStateData
}

type stateLogMetricsConfig struct {

	// MeterProvider sets the metric.MeterProvider.  If nil, the global
	// Provider will be used.
	MeterProvider metric.MeterProvider
	OCCName       string
}

//Interface define configuration parameters for statelog metrics agent
type Option interface {
	apply(*stateLogMetricsConfig)
}

//Define confuration for metric Provider Option
type MetricProviderOption struct {
	metric.MeterProvider
}

//Implement apply function in to configure meter provider
func (o MetricProviderOption) apply(c *stateLogMetricsConfig) {
	if o.MeterProvider != nil {
		c.MeterProvider = o.MeterProvider
	}
}

//Define Option for OCCName
type OCCNameOption string

const defaultOCCName string = "occ"

func (occName OCCNameOption) apply(c *stateLogMetricsConfig) {
	if occName != "" {
		c.OCCName = string(occName)
	}
}

//Create StateLogMetrics with OCC Name
func WithOCCName(occName string) Option {
	return OCCNameOption(occName)
}

//Create StateLogMetrics with provided meter Provider
func WitthMetricProvider(provider metric.MeterProvider) Option {
	return MetricProviderOption{provider}
}

// newConfig computes a config from the supplied Options.
func newConfig(opts ...Option) stateLogMetricsConfig {
	statesConfig := stateLogMetricsConfig{
		MeterProvider: global.MeterProvider(),
		OCCName:       defaultOCCName,
	}

	for _, opt := range opts {
		opt.apply(&statesConfig)
	}
	return statesConfig
}

// Start initializes reporting of stateLogMetrics using the supplied config.
func StartMetricsCollection(stateLogDataChan <-chan WorkersStateData, opt ...Option) error {
	stateLogMetricsConfig := newConfig(opt...)

	//Verfication of config data
	if stateLogMetricsConfig.OCCName == "" {
		stateLogMetricsConfig.OCCName = defaultOCCName
	}

	if stateLogMetricsConfig.MeterProvider == nil {
		stateLogMetricsConfig.MeterProvider = global.MeterProvider()
	}

	//Initialize statelog mterics
	stateLogMetrics := &StateLogMetrics{
		meter: stateLogMetricsConfig.MeterProvider.Meter("occ-statelog-data",
			metric.WithInstrumentationVersion("v1.0")),
		metricsConfig:  stateLogMetricsConfig,
		mStateDataChan: stateLogDataChan,
	}

	//Registers instrumentation for metrics
	return stateLogMetrics.register()
}

// Define Instrumentation for each metrics and register with StateLogMetrics
func (stateLogMetrics *StateLogMetrics) register() error {

	//"init", "acpt", "wait", "busy", "schd", "fnsh", "quce", "asgn", "idle", "bklg", "strd", "cls"}
	var (
		err error

		initState asyncint64.Gauge
		acptState asyncint64.Gauge
		waitState asyncint64.Gauge
		busyState asyncint64.Gauge
		schdState asyncint64.Gauge
		fnshState asyncint64.Gauge
		quceState asyncint64.Gauge
		asgnState asyncint64.Gauge
		idleState asyncint64.Gauge
		bklgState asyncint64.Gauge
		strdState asyncint64.Gauge

		//This lock prevents a race between batch observer and instrument registration
		lock sync.Mutex
	)

	lock.Lock()
	defer lock.Unlock()

	if initState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		INIT_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in init state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for init state", err)
		return err
	}

	if acptState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		ACCPT_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in accept state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for accept state", err)
		return err
	}

	if waitState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		WAIT_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in wait state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for wait state", err)
		return err
	}

	if busyState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		BUSY_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in busy state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for busy state", err)
		return err
	}

	if schdState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		SCHEDULED_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in scheduled state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for scheduled state", err)
		return err
	}

	if fnshState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		FINISHED_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in finished state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for finished state", err)
		return err
	}

	if quceState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		QUIESCED_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in quiesced state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for quiesced state", err)
		return err
	}

	if asgnState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		ASSIGNED_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in assigned state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for assigned state", err)
		return err
	}

	if idleState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		IDLE_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in idle state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for idle state", err)
		return err
	}

	if bklgState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		BACKLOG_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of workers in backlog state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for backlog state", err)
		return err
	}

	if strdState, err = stateLogMetrics.meter.AsyncInt64().Gauge(
		STRD_CONN_COUNT_METRIC,
		instrument.WithDescription("Number of connections in stranded state"),
	); err != nil {
		logger.GetLogger().Log(logger.Alert, "Failed to register guage metric for stranded state", err)
		return err
	}

	err = stateLogMetrics.meter.RegisterCallback(
		[]instrument.Asynchronous{
			initState,
			acptState,
			waitState,
			busyState,
			schdState,
			fnshState,
			quceState,
			asgnState,
			idleState,
			bklgState,
			strdState,
		}, func(ctx context.Context) {
			lock.Lock()
			defer lock.Unlock()

			//Infinite loop read through the channel and send metrics
			for {
				select {
				case workersState, more := <-stateLogMetrics.mStateDataChan:
					if !more { // TODO:: check zero value for workersState
						logger.GetLogger().Log(logger.Info, "Statelog metrics data channel 'mStateDataChan' has been closed.")
						return
					}

					commonLabels := []attribute.KeyValue{
						attribute.String("Application", stateLogMetrics.metricsConfig.OCCName),
						attribute.Int("ShardId", workersState.ShardId),
						attribute.Int("HeraWorkerType", int(workersState.WorkerType)),
						attribute.Int("InstanceId", workersState.InstanceId),
					}

					//Observe states data
					// 1. Worker States
					initState.Observe(ctx, int64(workersState.StateData["init"]), commonLabels...)
					acptState.Observe(ctx, int64(workersState.StateData["acpt"]), commonLabels...)
					waitState.Observe(ctx, int64(workersState.StateData["wait"]), commonLabels...)
					busyState.Observe(ctx, int64(workersState.StateData["busy"]), commonLabels...)
					schdState.Observe(ctx, int64(workersState.StateData["schd"]), commonLabels...)
					fnshState.Observe(ctx, int64(workersState.StateData["fnsh"]), commonLabels...)
					quceState.Observe(ctx, int64(workersState.StateData["quce"]), commonLabels...)

					// 2. Connection States
					asgnState.Observe(ctx, int64(workersState.StateData["asgn"]), commonLabels...)
					idleState.Observe(ctx, int64(workersState.StateData["idle"]), commonLabels...)
					bklgState.Observe(ctx, int64(workersState.StateData["bklg"]), commonLabels...)
					strdState.Observe(ctx, int64(workersState.StateData["strd"]), commonLabels...)
				default:
					return
				}
			}

		})

	if err != nil {
		return err
	}
	return nil
}
