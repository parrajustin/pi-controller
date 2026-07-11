import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http';
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-http';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { MeterProvider, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { LoggerProvider, BatchLogRecordProcessor } from '@opentelemetry/sdk-logs';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { trace, metrics } from '@opentelemetry/api';
import { logs } from '@opentelemetry/api-logs';

const resource = resourceFromAttributes({
  [ATTR_SERVICE_NAME]: 'display_control_web',
});

// Tracing
if (process.env.NODE_ENV !== 'test') {
  const provider = new WebTracerProvider({ 
    resource,
    spanProcessors: [
      new BatchSpanProcessor(new OTLPTraceExporter())
    ]
  });
  provider.register({
    contextManager: new ZoneContextManager(),
  });

  // Metrics
  const meterProvider = new MeterProvider({ 
    resource,
    readers: [
      new PeriodicExportingMetricReader({
        exporter: new OTLPMetricExporter(),
        exportIntervalMillis: 10000,
      })
    ]
  });
  metrics.setGlobalMeterProvider(meterProvider);

  // Logs
  const loggerProvider = new LoggerProvider({ 
    resource,
    processors: [
      new BatchLogRecordProcessor({ exporter: new OTLPLogExporter() })
    ]
  });
  logs.setGlobalLoggerProvider(loggerProvider);

  // Instrumentations
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
    ],
  });
}

export const tracer = trace.getTracer('display_control_web');
export const meter = metrics.getMeter('display_control_web');
export const logger = logs.getLogger('display_control_web');
