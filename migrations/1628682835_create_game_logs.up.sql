CREATE TABLE game_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    server_timestamp DATETIME NOT NULL,
    remote_addr TEXT NOT NULL,
    game_name VARCHAR(32) NOT NULL,
    payload JSON NOT NULL,
    PRIMARY KEY (id),
    KEY idx_game_name (game_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
