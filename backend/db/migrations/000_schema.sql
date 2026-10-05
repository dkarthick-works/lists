-- One-time provisioning, run by a database admin (the app role can neither
-- create schemas nor install extensions, so the server does not execute this file):
--   CREATE ROLE lists_app LOGIN PASSWORD '...';
CREATE SCHEMA IF NOT EXISTS lists AUTHORIZATION lists_app;

-- Trigram matching for list search. Installed in "public" so its operators
-- are on the app role's default search_path.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
