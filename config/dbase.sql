create database alertstrap;

create table if not exists mon_alerts (
  `alert_id`      varchar(50) not null,
  `group_id`      varchar(50) not null,
  `state`         varchar(10) not null,
  `active_at`     bigint(20) default 0,
  `starts_at`     bigint(20) default 0,
  `ends_at`       bigint(20) default 0,
  `repeat`        int default 1,
  `change_st`     int default 0,
  `labels`        json,
  `annotations`   json,
  `generator_url` varchar(1500),
  unique key IDX_mon_alerts_alert_id (alert_id),
  key IDX_mon_alerts_ends_at (ends_at),
  key IDX_mon_alerts_group_id_ends_at (group_id,ends_at)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists mon_actions (
  `action_id`     bigint(20),
  `login`         varchar(100) not null,
  `text`          text not null,
  `labels`        json,
  `generator_url` varchar(1500),
  `created`       bigint(20) default 0,
  primary key IDX_mon_actions_action_id (action_id)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;

create table if not exists mon_users (
  `login`         varchar(100) not null,
  `email`         varchar(100),
  `name`          varchar(150),
  `password`      varchar(100) not null,
  `token`         varchar(100) not null,
  unique key IDX_mon_users_login (login)
) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci;


--alter table mon_alerts rename column `status` TO `state`;
