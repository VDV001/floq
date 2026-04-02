CREATE TYPE step_channel AS ENUM ('email', 'telegram', 'phone_call');

ALTER TABLE sequence_steps
    ADD COLUMN channel step_channel NOT NULL DEFAULT 'email';

ALTER TABLE outbound_messages
    ADD COLUMN channel step_channel NOT NULL DEFAULT 'email';
