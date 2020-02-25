--create table if not exists showcase.mon_hosts (
--  host_id       varchar(50) not null,
--  serv_serv_id  varchar(50) not null,
--  host          varchar(100) not null,
--  name          varchar(150) not null,
--  primary key (host_id)
--) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

--create table if not exists showcase.mon_services (
--  serv_id       varchar(50) not null,
--  serv_serv_id  varchar(50) not null,
--  host          varchar(100) not null,
--  name          varchar(150) not null,
--  primary key (serv_id)
--) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

--create table if not exists showcase.mon_tasks (
--  mgrp_id varchar(50) not null,
--  task_key varchar(10),
--  date datetime default now(),
--  unique key IDX_mon_alerts_mgrp_id (mgrp_id)
--) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists alerts.mon_alerts (
  alert_id      varchar(50) not null,
  group_id      varchar(50) not null,
  status        varchar(10) not null,
  starts_at     bigint(20) default 0,
  ends_at       bigint(20) default 0,
  duplicate     int default 1,
  labels        json,
  annotations   json,
  generator_url varchar(400),
  unique key IDX_mon_alerts_alert_id (alert_id),
  key IDX_mon_alerts_ends_at (ends_at),
  key IDX_mon_alerts_group_id_ends_at (group_id,ends_at)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create or replace view alerts.mon_v_alerts as
select
  a.alert_id,
  a.group_id,
  a.status,
  a.starts_at,
  a.ends_at,
  a.duplicate,
  a.labels,
  a.annotations
from alerts.mon_alerts as a
join (
  select alert_id, max(ends_at) as ends_at from alerts.mon_alerts
  group by alert_id
) as a2 on a.alert_id = a2.alert_id
where a.ends_at > UNIX_TIMESTAMP() - 1800;

  --date_format(a.starts_at, '%Y-%m-%dT%H:%i:%s') as starts_at,
  --date_format(a.ends_at, '%Y-%m-%dT%H:%i:%s') as ends_at,

--create or replace view showcase.mon_v_hosts as
--select
--  hosts.host,
--  group_concat(hosts.serv_serv_id) as serv_id
--from showcase.mon_hosts as hosts group by host;
