BEGIN;
CREATE TABLE migration_state (
	name TEXT NOT NULL,
	time TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL,
	who TEXT DEFAULT CURRENT_USER NOT NULL,
	PRIMARY KEY (name)
);

CREATE TABLE migration_log (
  id SERIAL PRIMARY KEY,
  time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  name TEXT NOT NULL,
  op TEXT NOT NULL,
  who TEXT NOT NULL DEFAULT CURRENT_USER
);

CREATE OR REPLACE FUNCTION record_migration() RETURNS trigger AS $$
BEGIN
	IF TG_OP='DELETE' THEN
		INSERT INTO migration_log (name, op) VALUES (
			OLD.name,
			TG_OP
		);
		RETURN OLD;
	ELSE
		INSERT INTO migration_log (name, op) VALUES (
          NEW.name,
          TG_OP
		);
		RETURN NEW;
	END IF;
END;
$$ language plpgsql;

CREATE TRIGGER record_migration AFTER INSERT OR UPDATE OR DELETE ON migration_state
  FOR EACH ROW EXECUTE PROCEDURE record_migration();

INSERT INTO migration_state(name) VALUES ('00001_init');
COMMIT;
