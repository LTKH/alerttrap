create table if not exists showcase.mon_hosts (
  host_id       varchar(50) not null,
  serv_serv_id  varchar(50) not null,
  host          varchar(100) not null,
  name          varchar(150) not null,
  primary key (host_id)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists showcase.mon_services (
  serv_id       varchar(50) not null,
  serv_serv_id  varchar(50) not null,
  host          varchar(100) not null,
  name          varchar(150) not null,
  primary key (serv_id)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists showcase.mon_tasks (
  mgrp_id varchar(50) not null,
  task_key varchar(10),
  date datetime default now(),
  unique key IDX_mon_alerts_mgrp_id (mgrp_id)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists messages.mon_alerts (
  mess_id   varchar(50) not null,
  mgrp_id   varchar(50) not null,
  host      varchar(100) not null,
  severity  varchar(10) not null,
  sv_level  int default 0,
  ts_min    datetime default now(),
  ts_max    datetime default now(),
  ts_unix   bigint default 0,
  text      text,
  duplicate int default 1,
  port_id   int,
  appl_id   varchar(10),
  instance  varchar(50),
  mib       varchar(250),
  param     varchar(255),
  object    varchar(350),
  short_oid varchar(200),
  full_oid  varchar(1500),
  stand     varchar(50),
  zone      varchar(50),
  place     varchar(50),
  url       varchar(500),
  unique key IDX_mon_alerts_mess_id (mess_id),
  key IDX_mon_alerts_mgrp_id (mgrp_id),
  key IDX_mon_alerts_ts_max (ts_max),
  key IDX_mon_alerts_mgrp_id_ts_max (mgrp_id,ts_max)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create or replace view showcase.mon_v_hosts as
select
  hosts.host,
  group_concat(hosts.serv_serv_id) as serv_id
from showcase.mon_hosts as hosts group by host;

create or replace view messages.mon_v_alerts as
select
  alerts.mess_id,
  alerts.mgrp_id,
  alerts.host,
  alerts.severity,
  alerts.sv_level,
  date_format(alerts.ts_min, '%Y-%m-%dT%H:%i:%s') as ts_min,
  date_format(alerts.ts_max, '%Y-%m-%dT%H:%i:%s') as ts_max,
  alerts.ts_unix,
  alerts.text,
  alerts.duplicate,
  alerts.port_id,
  alerts.appl_id,
  alerts.instance,
  alerts.mib,
  alerts.param,
  alerts.object,
  alerts.stand,
  alerts.zone,
  alerts.place,
  alerts.url
from messages.mon_alerts as alerts
inner join (
  select mgrp_id, max(ts_max) as ts_max from messages.mon_alerts
  where ts_unix > UNIX_TIMESTAMP() - 1800 group by mgrp_id
) as alerts2 on alerts.mgrp_id = alerts2.mgrp_id and alerts.ts_max = alerts2.ts_max
where alerts.ts_unix > UNIX_TIMESTAMP() - 1800;
