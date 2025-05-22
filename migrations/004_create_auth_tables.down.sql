-- +migrate Down
DROP TABLE login_attempts;
DROP TABLE sessions;
DROP TABLE users; 