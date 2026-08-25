CREATE TABLE `evidence_files` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`sighting_id` integer NOT NULL,
	`object_key` text NOT NULL,
	`original_name` text NOT NULL,
	`mime_type` text NOT NULL,
	`size_bytes` integer NOT NULL,
	`sha256` text NOT NULL,
	`uploader` text NOT NULL,
	`created_at` text DEFAULT 'CURRENT_TIMESTAMP' NOT NULL
);
--> statement-breakpoint
CREATE TABLE `submission_limits` (
	`identity` text PRIMARY KEY NOT NULL,
	`window_started_at` integer NOT NULL,
	`count` integer DEFAULT 0 NOT NULL
);
--> statement-breakpoint
ALTER TABLE `sightings` ADD `location_precision` text DEFAULT 'approximate' NOT NULL;