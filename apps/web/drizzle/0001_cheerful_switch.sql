CREATE TABLE `moderation_events` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`sighting_id` integer NOT NULL,
	`old_status` text NOT NULL,
	`new_status` text NOT NULL,
	`reviewer` text NOT NULL,
	`note` text,
	`created_at` text DEFAULT 'CURRENT_TIMESTAMP' NOT NULL
);
