create table books (
id integer primary key,
created_on timestamp not null default (datetime()),
updated_on timestamp not null default (datetime()),
author text not null,
series text,
title text not null,
extension text not null,
tags text,
original_filename text not null,
filename text not null,
file_size integer not null,
file_mtime timestamp not null,
hash text not null unique,
regexp_name text not null,
template_override text,
source text
);
