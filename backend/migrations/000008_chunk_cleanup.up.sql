-- document_chunks bersifat polymorphic (tanpa FK), jadi hapus chunk saat sumbernya dihapus lewat trigger.
CREATE FUNCTION delete_source_chunks() RETURNS trigger AS $$
BEGIN
    DELETE FROM document_chunks WHERE source_type = TG_ARGV[0] AND source_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_job_postings_delete_chunks
    AFTER DELETE ON job_postings
    FOR EACH ROW EXECUTE FUNCTION delete_source_chunks('job_posting');

CREATE TRIGGER trg_learning_resources_delete_chunks
    AFTER DELETE ON learning_resources
    FOR EACH ROW EXECUTE FUNCTION delete_source_chunks('learning_resource');

-- Bersihkan chunk yatim yang sudah terlanjur ada.
DELETE FROM document_chunks c
WHERE (c.source_type = 'job_posting' AND NOT EXISTS (SELECT 1 FROM job_postings j WHERE j.id = c.source_id))
   OR (c.source_type = 'learning_resource' AND NOT EXISTS (SELECT 1 FROM learning_resources r WHERE r.id = c.source_id));
