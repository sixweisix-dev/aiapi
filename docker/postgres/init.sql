-- PostgreSQL initialization script
-- This script runs when the PostgreSQL container starts

-- Create database if it doesn't exist
SELECT 'CREATE DATABASE ai_gateway'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'ai_gateway')\gexec

-- Connect to the new database
\c ai_gateway

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";