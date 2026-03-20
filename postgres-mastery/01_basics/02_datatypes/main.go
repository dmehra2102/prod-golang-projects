package main

import (
	"context"

	"github.com/dmehra2102/prod-golang-projects/postgres-mastery/internal/db"
	"github.com/dmehra2102/prod-golang-projects/postgres-mastery/internal/logger"
)

const schema = `
	-- Drop everything so this lesson is re-runnable
	DROP TABLE IF EXISTS type_demo CASCADE;
	DROP TYPE IF EXISTS mood CASCADE;

	-- ------ ENUM TYPE -------------------------
	-- Enums enforce valid values at the database level.
	-- They're stored efficiently (4 bytes, like integer).
	CREATE TYPE mood AS ENUM ('happy', 'neutral', 'sad', 'excited');

	-- ------ MAIN DEMO TABLE -------------------
	CREATE TABLE type_demo (
		id 				BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

	-- ------- TEXT TYPES -------------------------
		name_text 		TEXT NOT NULL,
		name_varchar 	VARCHAR(50) NOT NULL,
		fixed_char 		CHAR(3) NOT NULL,

	-- -------- NUMERIC TYPES ---------------------
		count_small		SMALLINT,
		count_int 		INTEGER,
		count_big		BIGINT,
		price			NUMERIC(12, 2),
		ratio			DOUBLE PRECISION,
		ration_real		REAL,

	-- ──------- BOOLEAN ───────────────────────────────
		is_active       BOOLEAN NOT NULL DEFAULT true,

	-- ------- DATE/TIME ----------------------------
		birth_date 		DATE,
		start_time 		TIME,
		created_at		TIMESTAMP,
		updated_at		TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		duration 		INTERVAL,

	-- ──------- UUID ──────────────────────────────────
    -- uuid-ossp extension: gen_random_uuid() is built-in since PG13
		external_id     UUID NOT NULL DEFAULT gen_random_uuid(),

	-- --------- ARRAYS -------------------------------
		tags 			TEXT[],
		scores			INTEGER[],

	-- --------- JSONB -------------------------------
		metadata 		JSONB,
	
	-- --------- NETWORK TYPES -----------------------
		ip_address 		INET,
		network			CIRD,
	
	-- ── RANGE TYPES ─────────────────────────────────────────	
		valid_period    TSTZRANGE,

    -- ── ENUM ────────────────────────────────────────────────
		current_mood    mood
	)
`

func main() {
	logger.Header("MODULE 1 - LESSON 2", "PostgreSQL Data Types")

	ctx := context.Background()
	conn := db.MustConnect(ctx)
	defer conn.Close(ctx)

	// Setup schema
	logger.Lesson("Setup — Creating type_demo table")
	logger.SQL(schema)
	if _, err := conn.Exec(ctx, schema); err != nil {
		logger.Fatal(err)
		return
	}
	logger.Success("Schema created.")
}