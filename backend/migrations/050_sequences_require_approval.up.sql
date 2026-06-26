-- Outbound HITL (#180): per-sequence approve-before-send gate. When true,
-- every message this sequence launches starts as a draft awaiting operator
-- approval — overriding autopilot, which otherwise auto-approves at launch.
-- Default false preserves the prior behaviour (autopilot alone decides), so
-- existing sequences are unchanged.
ALTER TABLE sequences ADD COLUMN require_approval boolean NOT NULL DEFAULT false;
