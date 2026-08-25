CREATE TABLE `backup_runs` (`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,`kind` text NOT NULL,`status` text NOT NULL,`object_key` text,`sha256` text,`size_bytes` integer,`item_count` integer,`evidence_count` integer,`evidence_bytes` integer,`initiated_by` text NOT NULL,`error_code` text,`started_at` text DEFAULT CURRENT_TIMESTAMP NOT NULL,`completed_at` text);
--> statement-breakpoint
CREATE INDEX `backup_runs_status_idx` ON `backup_runs` (`status`,`completed_at`);
--> statement-breakpoint
CREATE TABLE `admin_audit_events` (`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,`actor` text NOT NULL,`action` text NOT NULL,`target_type` text,`target_id` text,`metadata_json` text,`created_at` text DEFAULT CURRENT_TIMESTAMP NOT NULL);
--> statement-breakpoint
CREATE INDEX `admin_audit_events_created_idx` ON `admin_audit_events` (`created_at`,`id`);
