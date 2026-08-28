# Log-based metrics.
#
# The 5xx alert tells you the error rate is up; the Cloud Run request metric
# it reads only knows HTTP status. It cannot say which endpoint broke or
# why, so acting on that alert meant opening Logs Explorer and starting a
# search. This turns the structured logging the API already emits into a
# metric, so the answer is on the dashboard next to the symptom.
#
# It works because every 5xx goes through serverError() in
# services/api/internal/httpapi/logging.go, which logs the real cause with
# the route and request ID at ERROR severity, and because production output
# is Cloud Logging JSON — the fields below are real fields, not text this
# has to parse out of a message.

resource "google_logging_metric" "api_errors" {
  project = var.project_id
  name    = "flockwatch/api_errors"

  description = "Application errors logged by the API, labelled by route and client-facing message."

  # severity>=ERROR rather than a text match: panics, encode failures, and
  # handler errors all land here without needing to enumerate them, and a
  # new error site is included automatically instead of being silently
  # missed until someone remembers to update a filter.
  filter = <<-EOT
    resource.type="cloud_run_revision"
    resource.labels.service_name="flockwatch-api"
    severity>=ERROR
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    unit        = "1"

    labels {
      key         = "route"
      value_type  = "STRING"
      description = "chi route pattern, e.g. /deployments/{id}. Empty for panics and startup failures, which have no route."
    }

    labels {
      key         = "message"
      value_type  = "STRING"
      description = "The client-facing message, e.g. 'could not load deployments'."
    }
  }

  # Deliberately NOT extracting the underlying error string. That field
  # carries database messages with table and column names — unbounded
  # cardinality, which Cloud Monitoring charges for and eventually rejects,
  # and needless duplication of content that already lives in the log entry.
  # The client-facing message is a small fixed set and is the right grain
  # for "what kind of thing is failing".
  label_extractors = {
    route   = "EXTRACT(jsonPayload.route)"
    message = "EXTRACT(jsonPayload.message)"
  }
}
