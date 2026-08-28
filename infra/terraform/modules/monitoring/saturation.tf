# Saturation alerts — warnings before an outage, not reports of one.
#
# The existing policies fire on failure that has already happened: the site
# is down, the API is throwing 5xx, the data went stale. These fire on the
# way there, while there is still time to raise a limit or find the cause.
#
# Both services run 512Mi / 1 CPU with max_instance_count = 3 and scale to
# zero. That shape drives every threshold and duration below:
#
#   - Scale-to-zero means cold starts. A cold start pins CPU near 100% for
#     seconds at a time, so every condition here needs a duration long
#     enough that a cold start cannot trip it. Short windows would produce
#     an alert on ordinary first-visitor traffic, and an alert that cries
#     wolf gets muted, which is worse than not having it.
#
#   - max_instance_count = 3 is a low ceiling. Hitting it is the most likely
#     real saturation path: Cloud Run queues requests once every instance is
#     busy, latency climbs, and clients eventually see 503s. That is the one
#     worth catching early, because the fix is a one-line limit change.
#
# Thresholds are intentionally below the point of failure. 85% memory is not
# an outage; it is the last comfortable moment to look.

# --- Memory ---------------------------------------------------------------

# The likeliest hard failure. Cloud Run kills a container that exceeds its
# memory limit, and the request in flight dies with it — visible to users as
# a 503 with nothing useful in the logs, because the process is gone before
# it can write anything.
resource "google_monitoring_alert_policy" "memory_saturation" {
  project      = var.project_id
  display_name = "Container memory approaching its limit"
  combiner     = "OR"

  conditions {
    display_name = "Memory above 85% of the limit"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_revision\"",
        "metric.type = \"run.googleapis.com/container/memory/utilizations\"",
      ])
      comparison      = "COMPARISON_GT"
      threshold_value = 0.85
      # Ten minutes. Memory pressure that matters is sustained; a transient
      # peak while serving a large response is not worth waking up for.
      duration = "600s"

      aggregations {
        alignment_period = "300s"
        # p99 across instances, then the worst service. A mean would hide a
        # single instance about to be killed behind two idle ones.
        per_series_aligner   = "ALIGN_PERCENTILE_99"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content = <<-EOT
      A Cloud Run container is running close to its 512Mi limit. Exceeding
      it means the container is killed mid-request, which surfaces as a 503
      with no error logged — the process dies before it can write one.

      Check the "Memory utilisation" panel on the Service health dashboard
      to see whether this is a step change (a new deployment) or a slow
      climb (a leak, or a response that grows with the dataset — the
      sitemap and the camera endpoints both build their whole response in
      memory).

      Raising the limit is `memory` in the cloud-run or static-site module.
    EOT
  }

  alert_strategy {
    auto_close = "86400s"
  }
}

# --- CPU ------------------------------------------------------------------

resource "google_monitoring_alert_policy" "cpu_saturation" {
  project      = var.project_id
  display_name = "Container CPU sustained near its limit"
  combiner     = "OR"

  conditions {
    display_name = "CPU above 80% of the limit"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_revision\"",
        "metric.type = \"run.googleapis.com/container/cpu/utilizations\"",
      ])
      comparison      = "COMPARISON_GT"
      threshold_value = 0.8
      # Fifteen minutes, the longest window here. CPU is the noisiest of
      # these signals — every cold start and every /cameras/clusters
      # aggregation spikes it briefly and legitimately.
      duration = "900s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_PERCENTILE_99"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content = <<-EOT
      Sustained high CPU. Unlike memory this rarely kills anything outright;
      it shows up as latency, and as instances scaling out toward the limit.

      The usual suspect on the API is a query doing more work than it
      should. /cameras/clusters aggregates the full camera table on a
      national-scale viewport (~370ms, full scan) and has no cache header,
      so a burst of traffic to the map's default view lands directly on the
      database.
    EOT
  }

  alert_strategy {
    auto_close = "86400s"
  }
}

# --- Autoscaling ceiling --------------------------------------------------

# The saturation path most likely to actually bite, because the ceiling is
# low (3) and nothing else reports it. Once every instance is busy Cloud Run
# queues requests rather than rejecting them, so the first symptom users see
# is latency, not errors — and by the time it becomes 503s the existing 5xx
# alert fires far too late to be a warning.
resource "google_monitoring_alert_policy" "instance_ceiling" {
  project      = var.project_id
  display_name = "Service pinned at its instance limit"
  combiner     = "OR"

  conditions {
    display_name = "Running at max_instance_count"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_revision\"",
        "metric.type = \"run.googleapis.com/container/instance_count\"",
        # Idle instances Cloud Run keeps warm are not saturation; only
        # instances actually serving count toward the ceiling.
        "metric.label.state = \"active\"",
      ])
      # 2.5 of a possible 3, not "equal to 3". The Monitoring API supports
      # only GT and LT — no GE — but a mean above 2.5 is the better signal
      # regardless: it catches a service that keeps hitting the ceiling and
      # falling back, which is the state just before sustained saturation,
      # rather than only the moment it is pinned there.
      comparison      = "COMPARISON_GT"
      threshold_value = 2.5
      duration        = "600s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_MEAN"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content = <<-EOT
      A service has been at its 3-instance ceiling for ten minutes. Further
      requests queue, so latency rises before any error appears.

      If the traffic is legitimate, raise max_instance_count in the
      cloud-run or static-site module. If it isn't, check the "Responses by
      status class" panel — a flood of 4xx alongside this usually means a
      scraper rather than readers.
    EOT
  }

  alert_strategy {
    auto_close = "86400s"
  }
}

# --- Latency --------------------------------------------------------------

# The symptom a reader actually experiences, and the one signal here that
# catches saturation this file didn't anticipate — a slow database, a
# degraded upstream, a query that got expensive as the dataset grew.
resource "google_monitoring_alert_policy" "latency" {
  project      = var.project_id
  display_name = "API responses are slow"
  combiner     = "OR"

  conditions {
    display_name = "p95 latency above 3s"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_revision\"",
        "resource.label.service_name = \"flockwatch-api\"",
        "metric.type = \"run.googleapis.com/request_latencies\"",
      ])
      comparison = "COMPARISON_GT"
      # 3s. Well above the normal picture — /cameras/clusters is the
      # slowest endpoint at ~500ms including a full scan — but below the
      # point where a reader assumes the map is broken and leaves.
      threshold_value = 3000
      duration        = "600s"

      aggregations {
        alignment_period = "300s"
        # p95, not p99: on a low-traffic service p99 is often a single cold
        # start, and alerting on it would mean alerting on the first
        # visitor after a quiet spell.
        per_series_aligner   = "ALIGN_PERCENTILE_95"
        cross_series_reducer = "REDUCE_MEAN"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content = <<-EOT
      p95 API latency has been above 3 seconds for ten minutes.

      Check the Service health dashboard in this order: instance count (at
      the ceiling means queuing), then CPU and memory (saturation), then the
      latency panel itself — if p50 has risen with p95 everything is slower,
      which points at the database rather than at this service.

      Neon is not visible in Cloud Monitoring, so if the Cloud Run metrics
      all look normal, the database is the next place to look.
    EOT
  }

  alert_strategy {
    auto_close = "86400s"
  }
}
