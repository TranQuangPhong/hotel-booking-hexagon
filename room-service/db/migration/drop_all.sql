-- Drops every room-service object, so the create_*.sql files can be applied from scratch.
-- Order matters: tables that reference rooms go first, enum types go after the tables that use them.
-- Re-create in this order: create_rooms_table.sql, create_inventories_table.sql, create_reservations_table.sql

DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS inventories;
DROP TABLE IF EXISTS inventory; -- old name of inventories
DROP TABLE IF EXISTS rooms;

DROP TYPE IF EXISTS reservation_status;
DROP TYPE IF EXISTS room_status;
DROP TYPE IF EXISTS room_type;
