-- +goose Up
-- The device's own clock at its newest report, so a report that was queued
-- offline can't overwrite newer state.
ALTER TABLE devices ADD COLUMN last_report_at timestamptz;
