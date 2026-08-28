# Cloud Monitoring dashboards.
#
# These are for looking, not for paging. The alert policies in main.tf are
# what wake someone up; a dashboard's job is to answer "what is going on"
# once you already know something is wrong, and to make normal look normal
# so abnormal is recognisable.
#
# Written with jsonencode() rather than a heredoc of literal JSON. The
# filter strings are full of embedded quotes, and escaping them by hand
# inside a JSON string inside HCL is how you end up with a dashboard that
# applies cleanly and renders nothing.
#
# Every metric here was checked against this project before being charted.
# A wrong metric type doesn't error — it produces an empty panel, which
# looks like "no traffic" rather than "wrong query", and that is a far worse
# failure than a broken apply.

locals {
  # Cloud Run request metrics are keyed on the revision, but the useful
  # grouping is the service: revisions change on every deploy, and a chart
  # grouped by revision turns into a new line each time you ship.
  run_revision = "resource.type=\"cloud_run_revision\""
}

resource "google_monitoring_dashboard" "service_health" {
  project = var.project_id

  dashboard_json = jsonencode({
    displayName = "FlockWatch — Service health"
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          xPos = 0, yPos = 0, width = 6, height = 4
          widget = {
            title = "Requests per second, by service"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "${local.run_revision} metric.type=\"run.googleapis.com/request_count\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["resource.label.\"service_name\""]
                    }
                  }
                }
                plotType = "LINE"
              }]
              yAxis = { label = "req/s", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 0, width = 6, height = 4
          widget = {
            # Split by class rather than charting only 5xx. A wall of 4xx is
            # a different problem (bad links, scrapers, a broken client) and
            # is invisible if you only ever plot server errors.
            title = "Responses by status class"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "${local.run_revision} metric.type=\"run.googleapis.com/request_count\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["metric.label.\"response_code_class\""]
                    }
                  }
                }
                plotType = "STACKED_BAR"
              }]
              yAxis = { label = "req/s", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 4, width = 6, height = 4
          widget = {
            # p50 alongside the tail. A p99 on its own can't distinguish
            # "everything got slower" from "a few requests are pathological",
            # and those have different causes.
            title = "API latency — p50 / p95 / p99"
            xyChart = {
              dataSets = [
                for p in ["50", "95", "99"] : {
                  legendTemplate = "p${p}"
                  timeSeriesQuery = {
                    timeSeriesFilter = {
                      filter = "${local.run_revision} resource.label.\"service_name\"=\"flockwatch-api\" metric.type=\"run.googleapis.com/request_latencies\""
                      aggregation = {
                        alignmentPeriod    = "60s"
                        perSeriesAligner   = "ALIGN_PERCENTILE_${p}"
                        crossSeriesReducer = "REDUCE_MEAN"
                      }
                    }
                  }
                  plotType = "LINE"
                }
              ]
              yAxis = { label = "ms", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 4, width = 6, height = 4
          widget = {
            # Both services scale to zero, so this is usually 0 or 1. It
            # earns its place as the cold-start explanation: a latency spike
            # with an instance count stepping up from zero is a cold start,
            # not a slow query.
            title = "Container instances"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "${local.run_revision} metric.type=\"run.googleapis.com/container/instance_count\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_MEAN"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["resource.label.\"service_name\""]
                    }
                  }
                }
                plotType = "STACKED_AREA"
              }]
              yAxis = { label = "instances", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 8, width = 6, height = 4
          widget = {
            title = "CPU utilisation (p99 across instances)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "${local.run_revision} metric.type=\"run.googleapis.com/container/cpu/utilizations\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_99"
                      crossSeriesReducer = "REDUCE_MAX"
                      groupByFields      = ["resource.label.\"service_name\""]
                    }
                  }
                }
                plotType = "LINE"
              }]
              yAxis = { label = "fraction of limit", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 8, width = 6, height = 4
          widget = {
            # Memory is the one most likely to bite: the API holds a pgx
            # pool and the sitemap builds its whole response in memory.
            title = "Memory utilisation (p99 across instances)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "${local.run_revision} metric.type=\"run.googleapis.com/container/memory/utilizations\""
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_99"
                      crossSeriesReducer = "REDUCE_MAX"
                      groupByFields      = ["resource.label.\"service_name\""]
                    }
                  }
                }
                plotType = "LINE"
              }]
              yAxis = { label = "fraction of limit", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 12, width = 12, height = 4
          widget = {
            # The panel that closes the loop on the 5xx alert. That alert
            # knows only HTTP status; this says which endpoint and what
            # kind of failure, straight from the structured logs, so acting
            # on it doesn't start with a search in Logs Explorer.
            title = "Application errors, by route and message"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_run_revision\" metric.type=\"logging.googleapis.com/user/${google_logging_metric.api_errors.name}\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_SUM"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields = [
                        "metric.label.\"route\"",
                        "metric.label.\"message\"",
                      ]
                    }
                  }
                }
                plotType = "STACKED_BAR"
              }]
              yAxis = { label = "errors", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 16, width = 12, height = 4
          widget = {
            # Grouped by checker location because a single failing region is
            # a network problem and every region failing is an outage. The
            # alert can't tell you which; this can.
            title = "Uptime checks passing, by probe location"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_FRACTION_TRUE"
                      crossSeriesReducer = "REDUCE_MEAN"
                      groupByFields = [
                        "metric.label.\"check_id\"",
                        "metric.label.\"checker_location\"",
                      ]
                    }
                  }
                }
                plotType = "LINE"
              }]
              yAxis = { label = "fraction passing", scale = "LINEAR" }
            }
          }
        },
      ]
    }
  })
}

resource "google_monitoring_dashboard" "data_pipeline" {
  project = var.project_id

  dashboard_json = jsonencode({
    displayName = "FlockWatch — Data pipeline"
    mosaicLayout = {
      columns = 12
      tiles = [
        {
          xPos = 0, yPos = 0, width = 12, height = 4
          widget = {
            # The headline number. /health/data returns 503 once the newest
            # import is more than nine days old, so this line dropping to
            # zero is the data going stale — regardless of whether the job
            # failed, succeeded while importing nothing, or never ran.
            title = "Data freshness check (0 = camera data is stale)"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" metric.label.\"check_id\"=\"${google_monitoring_uptime_check_config.data_freshness.uptime_check_id}\""
                    aggregation = {
                      alignmentPeriod    = "1800s"
                      perSeriesAligner   = "ALIGN_FRACTION_TRUE"
                      crossSeriesReducer = "REDUCE_MEAN"
                    }
                  }
                }
                plotType = "LINE"
              }]
              yAxis = { label = "fraction passing", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 0, yPos = 4, width = 6, height = 4
          widget = {
            # Weekly jobs, so expect a bar once every seven days and long
            # flat gaps. Emptiness here is normal; it is only meaningful
            # next to the freshness line above.
            title = "Job executions, by result"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_run_job\" metric.type=\"run.googleapis.com/job/completed_execution_count\""
                    aggregation = {
                      alignmentPeriod    = "3600s"
                      perSeriesAligner   = "ALIGN_SUM"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields = [
                        "resource.label.\"job_name\"",
                        "metric.label.\"result\"",
                      ]
                    }
                  }
                }
                plotType = "STACKED_BAR"
              }]
              yAxis = { label = "executions", scale = "LINEAR" }
            }
          }
        },
        {
          xPos = 6, yPos = 4, width = 6, height = 4
          widget = {
            # Task attempts rather than executions: a job that succeeds only
            # after retries is working but degrading, and the execution
            # count alone reports that as a clean success.
            title = "Task attempts, by result"
            xyChart = {
              dataSets = [{
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "resource.type=\"cloud_run_job\" metric.type=\"run.googleapis.com/job/completed_task_attempt_count\""
                    aggregation = {
                      alignmentPeriod    = "3600s"
                      perSeriesAligner   = "ALIGN_SUM"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields = [
                        "resource.label.\"job_name\"",
                        "metric.label.\"result\"",
                      ]
                    }
                  }
                }
                plotType = "STACKED_BAR"
              }]
              yAxis = { label = "attempts", scale = "LINEAR" }
            }
          }
        },
      ]
    }
  })
}
