ALTER TABLE `source_records` ADD `candidate_json` text DEFAULT '{}' NOT NULL;
--> statement-breakpoint
ALTER TABLE `source_records` ADD `reviewer` text;
--> statement-breakpoint
ALTER TABLE `source_records` ADD `reviewer_note` text;
--> statement-breakpoint
ALTER TABLE `source_records` ADD `reviewed_at` text;
