create table if not exists attachments (
    hash text primary key,
    size integer not null,
    content_type text not null,
    created_at text not null
);

create table if not exists chirp_attachments (
    chirp_id text not null references chirps(id) on delete cascade,
    attachment_hash text not null references attachments(hash) on delete cascade,
    filename text not null,
    created_at text not null,
    primary key (chirp_id, attachment_hash, filename)
);

create index if not exists chirp_attachments_chirp_idx on chirp_attachments(chirp_id);
create index if not exists chirp_attachments_hash_idx on chirp_attachments(attachment_hash);
