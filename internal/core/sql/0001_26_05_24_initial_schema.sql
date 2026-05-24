create table if not exists chirps (
    id text primary key,
    title text not null,
    text text not null,
    type text not null default 'text/markdown',
    created_at text not null,
    updated_at text not null
);

create table if not exists chirp_tags (
    chirp_id text not null references chirps(id) on delete cascade,
    tag text not null,
    primary key (chirp_id, tag)
);

create table if not exists chirp_fields (
    chirp_id text not null references chirps(id) on delete cascade,
    key text not null,
    value text not null,
    primary key (chirp_id, key)
);

create table if not exists chirp_refs (
    from_chirp_id text not null references chirps(id) on delete cascade,
    to_chirp_id text references chirps(id) on delete set null,
    ref_text text not null,
    resolved integer not null default 0,
    primary key (from_chirp_id, ref_text)
);

create index if not exists chirps_title_idx on chirps(title);
create index if not exists chirps_title_nocase_idx on chirps(title collate nocase);
create index if not exists chirp_refs_to_idx on chirp_refs(to_chirp_id);
create index if not exists chirp_refs_missing_idx on chirp_refs(ref_text) where resolved = 0;

create virtual table if not exists chirps_fts using fts5(id unindexed, title, text);

insert into chirps_fts (id, title, text)
    select c.id, c.title, c.text from chirps c
    where not exists (select 1 from chirps_fts f where f.id = c.id);
