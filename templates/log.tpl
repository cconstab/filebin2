{{ define "log" }}<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="icon" href="/static/img/favicon.png">
        <link rel="stylesheet" href="/static/css/bootstrap.min.css"/>
        <link rel="stylesheet" href="/static/css/fontawesome.all.min.css"/>
        <link rel="stylesheet" href="/static/css/upload.css"/>
        <link rel="stylesheet" href="/static/css/custom.css"/>
        <title>Filebin | Log</title>
    </head>
    <body class="container-xl">

        {{template "adminbar" .}}

        <h1>Log</h1>

	<p class="lead">Transactions logged for bin <a href="/{{ .Bin.Id }}">{{ .Bin.Id }}</a>.</p>

        <table class="table table-sm">
            <thead>
                <tr>
                    <th></th>
                    <th>Timestamp</th>
                    <th>Relative start time</th>
                    <th>IP</th>
                    <th>Operation</th>
                    <th>Object</th>
                    <th>Bytes</th>
                    <th>Status</th>
                    <th>Details</th>
                </tr>
            </thead>
            <tbody>
                {{ range $index, $value := .Transactions }}
                <tr>
                    <td>
                        {{ if eq .Method "POST" }}
                                <i class="fas fa-cloud-upload-alt text-primary"></i>
                        {{ end }}
                        {{ if eq .Method "GET" }}
                                <i class="fas fa-cloud-download-alt text-success"></i>
                        {{ end }}
                        {{ if eq .Method "DELETE" }}
                                <i class="fas fa-trash-alt text-danger"></i>
                        {{ end }}
                    </td>
                    <td>
                        {{ .Timestamp.Format "2006-01-02 15:04:05 UTC" }}
                    </td>
                    <td>
                        {{ .TimestampRelative }}
                    </td>
                    <td>{{ .IP }}</td>
                    <td>{{ .Type }}</td>
                    <td>
                        {{ if eq .Type "file-upload" }}
                            <a href="/{{ .BinId }}/{{ .Filename }}">{{ .Filename }}</a>
                        {{ end }}

                        {{ if eq .Type "file-delete" }}
                            {{ .Filename }}
                        {{ end }}

                        {{ if eq .Type "bin-delete" }}
                            -
                        {{ end }}

                        {{ if eq .Type "zip-download" }}
                            <a href="/archive/{{ .BinId }}/zip">Zip archive</a>
                        {{ end }}

                        {{ if eq .Type "tar-download" }}
                            <a href="/archive/{{ .BinId }}/tar">Tar archive</a>
                        {{ end }}

                        {{ if eq .Type "file-download" }}
                            <a href="/{{ .BinId }}/{{ .Filename }}">{{ .Filename }}</a>
                        {{ end }}
                    </td>
                    <td>
                        {{ .Bytes }}
                    </td>
                    <td>
                        {{ .Status }}
                    </td>
                    <td>
                        <a href="#" data-bs-toggle="collapse" data-bs-target="#collapse{{ $index }}" aria-expanded="false" aria-controls="collapse{{ $index }}"><i class="far fa-window-maximize"></i></a>
                    </td>
                </tr>
                <tr class="collapse" id="collapse{{ $index }}">
                    <td colspan="5">
                        <div class="card">
                            <div class="card-header">
                                    Request headers as submitted by the client
                            </div>
                            <div class="card-body pb-0">
                                    <code><pre>{{- .Trace -}}</pre></code>
                            </div>
                        </div>
                    </td>
                </tr>
                {{ end }}
            </tbody>
        </table>

        <script src="/static/js/popper.min.js"></script>
        <script src="/static/js/bootstrap.min.js"></script>
    </body>
</html>
{{ end }}
