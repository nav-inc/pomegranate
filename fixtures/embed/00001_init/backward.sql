BEGIN;
CREATE OR REPLACE FUNCTION no_rollback() RETURNS void AS $$
BEGIN
  RAISE 'Will not roll back 00001_init.  You must manually drop the migration_state and migration_log tables.';
END;
$$ LANGUAGE plpgsql;

SELECT no_rollback();
COMMIT;
