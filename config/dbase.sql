create database alertstrap;

create table if not exists mon_alerts (
  alert_id      varchar(50) not null,
  group_id      varchar(50) not null,
  status        varchar(10) not null,
  active_at     bigint(20) default 0,
  starts_at     bigint(20) default 0,
  ends_at       bigint(20) default 0,
  duplicate     int default 1,
  labels        json,
  annotations   json,
  generator_url varchar(1500),
  unique key IDX_mon_alerts_alert_id (alert_id),
  key IDX_mon_alerts_ends_at (ends_at),
  key IDX_mon_alerts_group_id_ends_at (group_id,ends_at)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists mon_users (
  login         varchar(100) not null,
  email         varchar(100),
  name          varchar(150),
  password      varchar(100) not null,
  token         varchar(100) not null,
  unique key IDX_mon_users_login (login)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;
