CREATE TABLE collection (
    id INTEGER PRIMARY KEY,
    label TEXT,
    created TEXT DEFAULT current_timestamp NOT NULL,
    modified TEXT DEFAULT current_timestamp NOT NULL,
    path TEXT GENERATED ALWAYS AS ('/org/freedesktop/secrets/collection/' || id) VIRTUAL
);

CREATE INDEX collection_path ON collection (path);

CREATE TABLE item (
    id INTEGER PRIMARY KEY,
    collection_id INTEGER NOT NULL,
    secret BLOB NOT NULL,
    label TEXT,
    created TEXT DEFAULT current_timestamp NOT NULL,
    modified TEXT DEFAULT current_timestamp NOT NULL,
    collection_path TEXT GENERATED ALWAYS AS ('/org/freedesktop/secrets/collection/' || collection_id) VIRTUAL,
    path TEXT GENERATED ALWAYS AS ('/org/freedesktop/secrets/collection/' || collection_id || '/' || id) VIRTUAL
);

CREATE INDEX item_path ON item (path);

CREATE TABLE item_attr (
    item_id INTEGER,
    key TEXT,
    value TEXT,
    PRIMARY KEY (item_id, key)
);

INSERT INTO collection (label) VALUES('default');
