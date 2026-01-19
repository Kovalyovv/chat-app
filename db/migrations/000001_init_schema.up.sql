create table rooms
(
    id          serial
        primary key,
    name        varchar(255)                           not null,
    invite_code varchar(16)                            not null
        unique,
    owner_id    integer                                not null,
    created_at  timestamp with time zone default now() not null
);

alter table rooms
    owner to postgres;

create table room_members
(
    user_id integer not null,
    room_id integer not null
        references rooms
            on delete cascade,
    primary key (user_id, room_id)
);

alter table room_members
    owner to postgres;

create table messages
(
    id           bigserial
        primary key,
    text         text,
    room_id      integer                                                    not null
        references rooms
            on delete cascade,
    user_id      integer                                                    not null,
    created_at   timestamp with time zone default now()                     not null,
    message_type varchar(20)              default 'TEXT'::character varying not null,
    metadata     jsonb
);

alter table messages
    owner to postgres;

create index messages_room_id_created_at_idx
    on messages (room_id asc, created_at desc);

create table room_read_states
(
    room_id                   integer                                not null
        references rooms
            on delete cascade,
    user_id                   integer                                not null,
    last_read_message_id      bigint                   default 0     not null,
    updated_at                timestamp with time zone default now() not null,
    last_delivered_message_id bigint                   default 0     not null,
    primary key (room_id, user_id)
);

alter table room_read_states
    owner to postgres;

