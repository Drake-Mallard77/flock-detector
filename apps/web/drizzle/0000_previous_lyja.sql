CREATE TABLE `contracts` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`agency` text NOT NULL,
	`state` text NOT NULL,
	`jurisdiction` text NOT NULL,
	`amount` integer,
	`camera_count` integer,
	`start_date` text,
	`end_date` text,
	`source_url` text,
	`status` text DEFAULT 'reported' NOT NULL
);
--> statement-breakpoint
CREATE TABLE `sightings` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`city` text NOT NULL,
	`state` text NOT NULL,
	`latitude` real NOT NULL,
	`longitude` real NOT NULL,
	`agency` text NOT NULL,
	`location` text NOT NULL,
	`source_url` text,
	`evidence_note` text,
	`status` text DEFAULT 'submitted' NOT NULL,
	`source_type` text DEFAULT 'community' NOT NULL,
	`submitted_by` text DEFAULT 'anonymous' NOT NULL,
	`created_at` text DEFAULT 'CURRENT_TIMESTAMP' NOT NULL
);
