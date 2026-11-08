#!/bin/sh
sed -i "s/\${PORT}/${PORT:-80}/g" /etc/nginx/conf.d/default.conf
exec nginx -g 'daemon off;'
