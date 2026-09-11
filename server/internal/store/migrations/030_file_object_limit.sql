-- Upgrade databases that already applied the original 8 MiB file limit.
ALTER TABLE file_objects DROP CONSTRAINT file_objects_size_check;
ALTER TABLE file_objects ADD CONSTRAINT file_objects_size_check CHECK (size BETWEEN 0 AND 16777216);
