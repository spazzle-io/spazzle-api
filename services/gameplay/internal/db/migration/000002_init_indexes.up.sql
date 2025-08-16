CREATE UNIQUE INDEX servers_name_unique_unarchived_idx
ON servers (name)
WHERE is_archived = false;

CREATE INDEX servers_created_at_id_desc
ON servers (created_at DESC, id DESC);

CREATE INDEX server_admins_server_added_at_user_desc
ON server_admins (server_id, added_at DESC, user_id DESC);

CREATE INDEX words_server_added_at_id_desc
ON words (server_id, added_at DESC, id DESC);
