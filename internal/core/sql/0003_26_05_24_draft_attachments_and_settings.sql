create table if not exists attachment_drafts (
    draft_id text not null,
    attachment_hash text not null references attachments(hash) on delete cascade,
    filename text not null,
    created_at text not null,
    primary key (draft_id, attachment_hash, filename)
);

create index if not exists attachment_drafts_draft_idx on attachment_drafts(draft_id);

create table if not exists app_settings (
    key text primary key,
    value text not null,
    updated_at text not null
);
