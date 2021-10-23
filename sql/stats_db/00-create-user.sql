CREATE extension ltree;
CREATE DATABASE stats_db;
\connect stats_db;
CREATE USER zchain_user WITH ENCRYPTED PASSWORD 'zchian';
GRANT ALL PRIVILEGES ON DATABASE stats_db TO zchain_user;