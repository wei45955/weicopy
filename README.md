# WeiCopy

自托管的跨设备剪贴板。没有账号。进入页面时选一个会话名，例如 `weicopy`，再选它保留多久。另一台设备进入同一个名字，就能看到里面的文本、图片和文件，也可以修改或删除。

## 会话

- 时长默认 30 分钟，也可以选更长，或者永久。时间从创建时算起，后来进入不会顺延。
- 密码可选，留空就是没有密码。知道名字就能进入。
- 这台浏览器会把最近一次会话记在 local storage。超时后再打开页面，会问你是重新初始化这个会话，还是开始一个新的。
- 重新初始化会清空旧内容，并按新的时长重新计时。
- 「离开」只断开这台设备。「结束会话」会立刻清空，所有设备都进不去。

## 运行

```bash
docker compose up -d --build
```

服务监听 `8080`，页面和 `/api` 在同一个端口。本仓库不再内置 Nginx。如果外面已经有 Nginx，把整站反代过来即可：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 50m;
}
```

数据在 Docker volume `weicopy-data` 和 `weicopy-uploads` 里。`MAX_UPLOAD_SIZE_MB` 默认 50。

## 命令行

会话名写在路径里。没有密码时不要带 `Authorization`。有密码时，头的值就是密码本身，不要加 `Bearer`。

```bash
curl -s -X POST http://127.0.0.1:8080/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{"name":"weicopy","password":"","ttl_minutes":30}'

curl -X POST http://127.0.0.1:8080/api/sessions/weicopy/items/text \
  -H "Content-Type: text/plain" \
  --data-binary "你好"

curl -X POST http://127.0.0.1:8080/api/sessions/weicopy/items/file \
  -F "file=@/path/to/file"

curl http://127.0.0.1:8080/api/sessions/weicopy/state

curl "http://127.0.0.1:8080/api/sessions/weicopy/items/latest?raw=1"
```

有密码时加上 `-H "Authorization: 密码"`。`ttl_minutes` 为 `0` 表示永久。更新文本用 `PUT /api/sessions/weicopy/items/ID`，JSON 正文是 `{"content":"..."}`。删除用 `DELETE /api/sessions/weicopy/items/ID`。结束整个会话用 `DELETE /api/sessions/weicopy`。
