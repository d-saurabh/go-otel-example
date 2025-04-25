# Observability with OpenTelemetry in a Go Application

This repository is a **comprehensive guide for developers** who want to master **telemetry in Go applications** using the OpenTelemetry SDKs. Whether you're new to observability or looking to enhance your skills, this project demonstrates how to seamlessly integrate **logs, traces, and metrics** into your application. It includes a fully functional observability stack with **Prometheus**, **Loki**, **Grafana**, and **Zipkin** for real-time monitoring, troubleshooting, and visualization.

---

## **Overview**

The goal of this project is to help developers understand how to:
- Instrument a Go application with OpenTelemetry for **logs**, **metrics**, and **traces**.
- Export telemetry data to an **OpenTelemetry Collector** for centralized processing.
- Visualize metrics and logs in **Grafana**, traces in **Zipkin**.

---

## Observability Concepts

### Traces

Traces represent the lifecycle of a request as it flows through the application. Each trace consists of spans, which are individual units of work (e.g., HTTP requests, database queries).

```plaintext

[ HTTP Request Received ]
           │
           ▼
[ Start Root Span (otelhttp) ]
           │
           ▼
[ Handler Execution ]
           │
           ▼
[ Start Child Span (e.g., DB Query) ]
           │
           ▼
[ End Child Span ]
           │
           ▼
[ End Root Span ]
```

**Gotchas**
- Avoid Over-Instrumentation
  Instrument only critical paths (e.g., HTTP handlers, DB queries). Instrumenting every function can lead to high costs and noisy data.

- Sampling 
  Use sampling strategies (e.g., probabilistic sampling) to reduce the volume of traces sent to the backend.

- Span Attributes 
  Limit the number of attributes per span to avoid bloating telemetry data.

### Metrics

Metrics provide aggregated data about the system's performance, such as request counts, latencies, and error rates.

```plaintext

[ HTTP Request Received ]
           │
           ▼
[ Record Metrics ]
  - Request Count
  - Request Duration
  - Error Count
           │
           ▼
[ Export Metrics to Backend ]
```

**Gotchas**
- Cardinality Explosion
  Avoid high-cardinality labels (e.g., user IDs, session IDs) in metrics. Use aggregated labels like status_code or endpoint.
- Granularity
  Choose an appropriate granularity for metrics. Too fine-grained metrics can increase storage costs.
- Aggregation
  Use pre-aggregated metrics (e.g., histograms) to reduce the volume of data sent to the backend.

### Logs

Logs capture detailed information about application events, such as errors, warnings, and debug messages.

```plaintext
[ Application Event ]
           │
           ▼
[ Generate Log Entry ]
  - Timestamp
  - Log Level (INFO, ERROR, etc.)
  - Message
  - Context (e.g., traceID, spanID)
           │
           ▼
[ Export Logs to Backend ]
```

**Gotchas**

- Log Levels
  Use appropriate log levels (e.g., DEBUG for development, INFO for production). Avoid excessive DEBUG logs in production.
- Structured Logging
  Use structured logs (e.g., JSON) to make logs easier to query and correlate with traces.
- Retention Policies
  Configure log retention policies to avoid excessive storage costs.


## **How to Run the Application**

Follow these steps to run the application and the observability stack using Docker Compose:

1. **Clone the Repository**:

```bash
   git clone https://github.com/d-saurabh/observability.git
   cd observability

---

2. **Build and Start the Services**:

Run the following command to build the Go application and start all services:

```bash
    docker-compose up --build
```

- This will start:
    - The Go application (my-app).
    - OpenTelemetry Collector.
    - Prometheus, Loki, Zipkin, and Grafana.

---

3. **Verify the Services**:

Check if all services are running:
```bash

    docker ps
```
---

4. **Verify the Logging BE ready**:
```bash
    curl http://localhost:3100/ready
```

This will check if the loki is running up, wait for sometime till this reponds with `ready`

---

5. **Send a Request to the Application**:

Use curl or a browser to send a request to the application:
```bash
    curl http://localhost:8080/hello/1
```

---

## **How to Visualize Telemetry Data**

Once the application and observability stack are running, you can visualize the telemetry data in the following tools:

---

### **1. Metrics in Prometheus and Grafana**

- **Grafana**:
  - Access Grafana at [http://localhost:3000](http://localhost:3000).
  - Steps to visualize metrics:
    1. Under `drilldown` feature, you can explore metrics.
    note: the prometheus is already added as a default datasource for metrics.
    2. Import pre-built dashboards or create custom dashboards to visualize metrics.

---

### **2. Traces in Zipkin**

- Access Zipkin at [http://localhost:9411](http://localhost:9411).
- Steps to visualize traces:
  1. Use the Zipkin UI to search for traces by service name, trace ID, or time range.
  2. View the trace details to analyze the flow of requests through the application.
  3. Identify bottlenecks or errors in the request lifecycle.

---

### **3. Logs in Loki and grafana**

- **Loki**:
  - Loki collects logs from the application and other services.

- **Grafana**:
  - Steps to visualize logs:
    1. [OPTIONAL] Add Loki as a data source in Grafana. note: this is already done
    2. Use the "Explore" tab in Grafana to query logs.
    3. Example query:
       ```logql
       {job="my-app"}
       ```
    4. Filter logs by labels such as `app`, `level`, or `traceID` to correlate logs with traces.

---

### **4. Example Workflow**

1. **Send a Request to the Application**:
   - Use `curl` or a browser to send a request to the application:
     ```bash
     curl http://localhost:8080/hello
     ```

2. **View Metrics**:
   - Open Prometheus or Grafana to view metrics like request counts and durations.

3. **View Traces**:
   - Open Zipkin to view the trace of the `/hello` request and analyze its lifecycle.

4. **View Logs**:
   - Open Grafana and query logs collected by Loki to debug or correlate with traces.

---

### **5. Stopping the Application**

To stop the application and all services, run:
```bash
    docker-compose down
```

## **Architecture Diagram**

Below is the architecture of the observability setup:

```plaintext

                         [ Application Startup ]
                                   │
                                   ▼
                        ┌────────────────────┐
                        │ Initialize Resources│
                        │ - Service Name      │
                        │ - Service Version   │
                        └────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────┐
                    │ Set Up Exporters        │
                    │ - Trace Exporter (OTLP) │
                    │ - Metric Exporter (OTLP)│
                    └─────────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────┐
                    │ Configure Providers     │
                    │ - Tracer Provider       │
                    │ - Meter Provider        │
                    └─────────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────┐
                    │ Instrumentation Setup   │
                    │ - Wrap HTTP Handlers    │
                    │   (otelhttp, etc.)      │
                    │ - Add Middleware        │
                    │   (e.g., Chi middleware)│
                    └─────────────────────────┘
                                   │
                                   ▼
                      ┌─────────────────────┐
                      │ Running Application │
                      └─────────────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────┐
                    │ HTTP Request Received   │
                    └─────────────────────────┘
                                   │
                                   ▼
             ┌─────────────────────────────────────┐
             │ Instrumentation Middleware (otelhttp)│
             │ - Starts a new Span                │
             │ - Records metrics (e.g., duration) │
             └─────────────────────────────────────┘
                                   │
                                   ▼
                      ┌─────────────────────┐
                      │ Handler Execution   │
                      └─────────────────────┘
                                   │
                                   ▼
                   ┌────────────────────────┐
                   │ End Span and Aggregate │
                   │ Telemetry Data         │
                   └────────────────────────┘
                                   │
                                   ▼
                   ┌────────────────────────┐
                   │ Export Telemetry Data  │
                   │ to Observability Backend│
                   └────────────────────────┘
```
