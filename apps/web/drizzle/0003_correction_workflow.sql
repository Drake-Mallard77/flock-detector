CREATE TABLE `correction_requests` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`sighting_id` integer NOT NULL,
	`request_type` text NOT NULL,
	`explanation` text NOT NULL,
	`source_url` text,
	`submitted_by` text NOT NULL,
	`status` text DEFAULT 'pending' NOT NULL,
	`reviewer` text,
	`reviewer_note` text,
	`created_at` text DEFAULT 'CURRENT_TIMESTAMP' NOT NULL,
	`reviewed_at` text
);
--> statement-breakpoint
CREATE INDEX `correction_requests_sighting_idx` ON `correction_requests` (`sighting_id`,`status`);
--> statement-breakpoint
CREATE TABLE `source_records` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`sighting_id` integer,
	`source_url` text NOT NULL,
	`retrieved_at` text NOT NULL,
	`content_sha256` text,
	`archive_url` text,
	`extracted_claim` text NOT NULL,
	`review_status` text DEFAULT 'pending' NOT NULL,
	`created_by` text NOT NULL,
	`created_at` text DEFAULT 'CURRENT_TIMESTAMP' NOT NULL
);
--> statement-breakpoint
CREATE INDEX `source_records_review_idx` ON `source_records` (`review_status`,`id`);
