-- The IP address a client last registered or polled from -- lets the
-- admin tell apart two pending clients with generic names/pairing
-- codes (e.g. two browser tabs both named "Kitchen") before approving
-- one. Refreshed on every registration and config poll, not just set
-- once, since DHCP leases and re-registrations can change it.
ALTER TABLE clients ADD COLUMN ip_address TEXT;
