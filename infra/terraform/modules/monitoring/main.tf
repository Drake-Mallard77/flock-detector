# Alerting exists here for one specific reason: every failure mode this
# project has is quiet. A failed weekly import doesn't error anywhere a
# human looks — the site keeps serving last week's data indefinitely, and
# for a project whose value is current, accurate records, silently stale is
# worse than visibly broken.

resource "google_monitoring_notification_channel" "email" {
  project      = var.project_id
  display_name = "FlockWatch alerts"
  type         = "email"

  labels = {
    email_address = var.alert_email
  }
}

# --- Scheduled jobs -------------------------------------------------------

# Fires when either weekly Cloud Run Job reports a failed task. Without this
# the import stops running and nothing says so.
resource "google_monitoring_alert_policy" "job_failed" {
  project      = var.project_id
  display_name = "Scheduled job failed"
  combiner     = "OR"

  conditions {
    display_name = "A Cloud Run Job task failed"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_job\"",
        "metric.type = \"run.googleapis.com/job/completed_task_attempt_count\"",
        "metric.label.result = \"failed\"",
      ])
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      duration        = "0s"

      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content = join("\n", [
      "A scheduled FlockWatch job failed.",
      "",
      "flockwatch-refresh-cameras re-imports ALPR camera locations from",
      "OpenStreetMap; flockwatch-derive-deployments proposes agency-level",
      "candidates for review. Neither failing breaks the site, but the data",
      "stops being refreshed and nothing else will tell you.",
      "",
      "Logs: https://console.cloud.google.com/run/jobs?project=${var.project_id}",
      "Both jobs are idempotent, so re-running is always safe.",
    ])
    mime_type = "text/markdown"
  }

  alert_strategy {
    # Jobs run weekly, so an incident that auto-closes in hours would shut
    # before anyone acts on it.
    auto_close = "604800s"
  }
}

# The inverse failure: the scheduler stops firing at all, so nothing fails
# because nothing runs. A job-failure alert cannot catch that.
#
# The obvious approach — alert on absence of the job metric — is not
# expressible: Cloud Monitoring caps metric-absence conditions at 23h30m and
# this job runs weekly. Measuring the outcome is better anyway. A job can run,
# report success, and still import nothing; what matters is whether the
# records are current. /health/data returns 503 once the newest import is
# older than nine days, and the uptime check below alerts on that.
resource "google_monitoring_uptime_check_config" "data_freshness" {
  project      = var.project_id
  display_name = "FlockWatch data freshness"
  timeout      = "10s"
  # Hourly rather than every five minutes: this measures a weekly job, so
  # tighter polling adds noise and cost without detecting anything sooner.
  period = "900s"

  http_check {
    path         = "/health/data"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = replace(replace(var.api_url, "https://", ""), "/", "")
    }
  }
}

resource "google_monitoring_alert_policy" "data_stale" {
  project      = var.project_id
  display_name = "Camera data is stale"
  combiner     = "OR"

  conditions {
    display_name = "Freshness check failing"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"uptime_url\"",
        "metric.type = \"monitoring.googleapis.com/uptime_check/check_passed\"",
        "metric.label.check_id = \"${google_monitoring_uptime_check_config.data_freshness.uptime_check_id}\"",
      ])
      comparison      = "COMPARISON_LT"
      threshold_value = 1
      # An hour of consecutive failures, so a transient API blip doesn't
      # read as stale data.
      duration = "3600s"

      aggregations {
        alignment_period     = "900s"
        per_series_aligner   = "ALIGN_FRACTION_TRUE"
        cross_series_reducer = "REDUCE_MEAN"
        group_by_fields      = ["resource.label.host"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  documentation {
    content   = <<-EOT
      The newest OpenStreetMap import is more than nine days old.

      The site is up and serving, but the camera data has stopped being
      refreshed — most likely the weekly Cloud Scheduler trigger or the
      Cloud Run Job has stopped running.

      Check: https://console.cloud.google.com/cloudscheduler
      The import is idempotent, so re-running it by hand is always safe.
    EOT
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "604800s"
  }
}

# --- Uptime ---------------------------------------------------------------

resource "google_monitoring_uptime_check_config" "site" {
  project      = var.project_id
  display_name = "FlockWatch site"
  timeout      = "10s"
  period       = "300s"

  http_check {
    path         = "/"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = replace(replace(var.site_url, "https://", ""), "/", "")
    }
  }
}

resource "google_monitoring_uptime_check_config" "api" {
  project      = var.project_id
  display_name = "FlockWatch API"
  timeout      = "10s"
  period       = "300s"

  http_check {
    # /health, not /healthz — Google's front end intercepts that exact path
    # before it reaches the container. See docs/ARCHITECTURE.md.
    path         = "/health"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = replace(replace(var.api_url, "https://", ""), "/", "")
    }
  }
}

resource "google_monitoring_alert_policy" "uptime" {
  project      = var.project_id
  display_name = "Site or API is down"
  combiner     = "OR"

  conditions {
    display_name = "Uptime check failing"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"uptime_url\"",
        "metric.type = \"monitoring.googleapis.com/uptime_check/check_passed\"",
      ])
      comparison      = "COMPARISON_LT"
      threshold_value = 1
      # Two consecutive failed windows, so a single blip from one probe
      # location doesn't page anyone.
      duration = "600s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_FRACTION_TRUE"
        cross_series_reducer = "REDUCE_MEAN"
        group_by_fields      = ["resource.label.host"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "86400s"
  }
}

# --- Application errors ---------------------------------------------------

# Cloud Run 5xx responses. Distinct from the uptime check: the service can be
# up and answering while a specific endpoint is broken for every visitor.
resource "google_monitoring_alert_policy" "api_errors" {
  project      = var.project_id
  display_name = "API returning server errors"
  combiner     = "OR"

  conditions {
    display_name = "5xx responses from the API"

    condition_threshold {
      filter = join(" AND ", [
        "resource.type = \"cloud_run_revision\"",
        "resource.label.service_name = \"flockwatch-api\"",
        "metric.type = \"run.googleapis.com/request_count\"",
        "metric.label.response_code_class = \"5xx\"",
      ])
      comparison = "COMPARISON_GT"
      # Not zero: an occasional 5xx during a cold start or deploy is normal,
      # and an alert that cries wolf gets muted, which is worse than none.
      threshold_value = 5
      duration        = "300s"

      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]

  alert_strategy {
    auto_close = "86400s"
  }
}
