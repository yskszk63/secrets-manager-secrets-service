CREATE TABLE collection (
    id INTEGER PRIMARY KEY,
    label TEXT,
    created TEXT DEFAULT current_timestamp NOT NULL,
    modified TEXT DEFAULT current_timestamp NOT NULL
);

CREATE TABLE item (
    id INTEGER PRIMARY KEY,
    collection_id INTEGER NOT NULL,
    secret BLOB NOT NULL,
    label TEXT,
    created TEXT DEFAULT current_timestamp NOT NULL,
    modified TEXT DEFAULT current_timestamp NOT NULL
);

CREATE TABLE item_attr (
    item_id INTEGER,
    key TEXT,
    value TEXT,
    PRIMARY KEY (item_id, key)
);

INSERT INTO collection (label) VALUES('default');
