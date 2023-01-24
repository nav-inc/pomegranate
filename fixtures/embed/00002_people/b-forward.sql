CREATE TABLE IF NOT EXISTS pets (
	id BIGSERIAL PRIMARY KEY,
  animal bigint references animal(id),
	name TEXT NOT NULL
);
