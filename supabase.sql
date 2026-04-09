drop table if exists public.tokens;

create table public.tokens (
  id text not null,
  email text null,
  token_data text not null,
  update_time timestamp with time zone null,
  constraint token_pkey primary key (id)
) TABLESPACE pg_default;

create index IF not exists idx_update_time on public.tokens using btree (update_time) TABLESPACE pg_default;

alter table public.tokens enable row level security;

grant select, insert, update on public.tokens to anon;

create policy "tokens_select_anon"
on public.tokens
for select
to anon
using (true);

create policy "tokens_insert_anon"
on public.tokens
for insert
to anon
with check (true);

create policy "tokens_update_anon"
on public.tokens
for update
to anon
using (true)
with check (true);