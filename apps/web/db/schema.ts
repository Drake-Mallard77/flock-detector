import { integer, real, sqliteTable, text } from "drizzle-orm/sqlite-core";

export const sightings = sqliteTable("sightings", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  city: text("city").notNull(), state: text("state").notNull(),
  latitude: real("latitude").notNull(), longitude: real("longitude").notNull(),
  agency: text("agency").notNull(), location: text("location").notNull(),
  sourceUrl: text("source_url"), evidenceNote: text("evidence_note"),
  status: text("status").notNull().default("submitted"),
  sourceType: text("source_type").notNull().default("community"),
  submittedBy: text("submitted_by").notNull().default("anonymous"),
  createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
  locationPrecision: text("location_precision").notNull().default("approximate"),
});

export const contracts = sqliteTable("contracts", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  agency: text("agency").notNull(), state: text("state").notNull(),
  jurisdiction: text("jurisdiction").notNull(), amount: integer("amount"),
  cameraCount: integer("camera_count"), startDate: text("start_date"),
  endDate: text("end_date"), sourceUrl: text("source_url"),
  status: text("status").notNull().default("reported"),
});

export const moderationEvents = sqliteTable("moderation_events", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  sightingId: integer("sighting_id").notNull(),
  oldStatus: text("old_status").notNull(),
  newStatus: text("new_status").notNull(),
  reviewer: text("reviewer").notNull(),
  note: text("note"),
  createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
});

export const evidenceFiles = sqliteTable("evidence_files", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  sightingId: integer("sighting_id").notNull(),
  objectKey: text("object_key").notNull(),
  originalName: text("original_name").notNull(),
  mimeType: text("mime_type").notNull(),
  sizeBytes: integer("size_bytes").notNull(),
  sha256: text("sha256").notNull(),
  uploader: text("uploader").notNull(),
  createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
});

export const submissionLimits = sqliteTable("submission_limits", {
  identity: text("identity").primaryKey(),
  windowStartedAt: integer("window_started_at").notNull(),
  count: integer("count").notNull().default(0),
});

export const correctionRequests = sqliteTable("correction_requests", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  sightingId: integer("sighting_id").notNull(),
  requestType: text("request_type").notNull(),
  explanation: text("explanation").notNull(),
  sourceUrl: text("source_url"),
  submittedBy: text("submitted_by").notNull(),
  status: text("status").notNull().default("pending"),
  reviewer: text("reviewer"),
  reviewerNote: text("reviewer_note"),
  createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
  reviewedAt: text("reviewed_at"),
});

export const sourceRecords = sqliteTable("source_records", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  sightingId: integer("sighting_id"),
  sourceUrl: text("source_url").notNull(),
  retrievedAt: text("retrieved_at").notNull(),
  contentSha256: text("content_sha256"),
  archiveUrl: text("archive_url"),
  extractedClaim: text("extracted_claim").notNull(),
  candidateJson: text("candidate_json").notNull(),
  reviewStatus: text("review_status").notNull().default("pending"),
  createdBy: text("created_by").notNull(),
  reviewer: text("reviewer"),
  reviewerNote: text("reviewer_note"),
  reviewedAt: text("reviewed_at"),
  createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
});

export const backupRuns = sqliteTable("backup_runs", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  kind: text("kind").notNull(), status: text("status").notNull(),
  objectKey: text("object_key"), sha256: text("sha256"),
  sizeBytes: integer("size_bytes"), itemCount: integer("item_count"),
  evidenceCount: integer("evidence_count"), evidenceBytes: integer("evidence_bytes"),
  initiatedBy: text("initiated_by").notNull(), errorCode: text("error_code"),
  startedAt: text("started_at").notNull().default("CURRENT_TIMESTAMP"), completedAt: text("completed_at"),
});

export const adminAuditEvents = sqliteTable("admin_audit_events", {
  id: integer("id").primaryKey({ autoIncrement: true }),
  actor: text("actor").notNull(), action: text("action").notNull(),
  targetType: text("target_type"), targetId: text("target_id"),
  metadataJson: text("metadata_json"), createdAt: text("created_at").notNull().default("CURRENT_TIMESTAMP"),
});
