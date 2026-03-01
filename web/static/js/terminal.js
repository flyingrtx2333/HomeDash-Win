
// 正确导入：使用 @xterm/ 前缀
import { Terminal } from 'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/+esm';
import { FitAddon } from 'https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.11.0/+esm';

let terminalWs = null;
let term = null;
let fitAddon = null;

function connectTerminal() {
    console.log('正在连接到终端...');

    if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
        terminalWs.close();
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/terminal`;

    updateTerminalStatus('connecting');

    if (!term) {
        term = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Consolas, "Courier New", monospace',
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#ffffff',
                selection: 'rgba(255, 255, 255, 0.3)',
                black: '#000000',
                red: '#cd3131',
                green: '#0dbc79',
                yellow: '#e5e510',
                blue: '#2472c8',
                magenta: '#bc3fbc',
                cyan: '#11a8cd',
                white: '#e5e5e5',
                brightBlack: '#666666',
                brightRed: '#f14c4c',
                brightGreen: '#23d18b',
                brightYellow: '#f5f543',
                brightBlue: '#3b8eea',
                brightMagenta: '#d670d6',
                brightCyan: '#29b8db',
                brightWhite: '#e5e5e5'
            },
            allowTransparency: false,
            scrollback: 5000,
            convertEol: true
        });

        fitAddon = new FitAddon();
        term.loadAddon(fitAddon);

        const container = document.getElementById('terminalContainer');
        term.open(container);

        window.addEventListener('resize', () => {
            if (fitAddon) fitAddon.fit();
        });

        term.onResize(({ cols, rows }) => {
            if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                terminalWs.send(JSON.stringify({
                    type: "resize",
                    cols: cols,
                    rows: rows
                }));
            }
        });

        term.onData(data => {
            if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                console.log('发送data:', data);
                terminalWs.send(data);
            }
        });
    }

    term.clear();

    terminalWs = new WebSocket(wsUrl);

    terminalWs.onopen = () => {
        updateTerminalStatus('connected');
        term.writeln("\x1b[32m[系统] 已连接到远程终端...\x1b[0m");
        term.focus();

        const doFitAndResize = () => {
            if (fitAddon) {
                fitAddon.fit();
                if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                    terminalWs.send(JSON.stringify({
                        type: "resize",
                        cols: term.cols,
                        rows: term.rows
                    }));
                }
            }
        };

        // 多次尝试，确保首次加载成功
        doFitAndResize();
        setTimeout(doFitAndResize, 100);
        setTimeout(doFitAndResize, 300);
        setTimeout(doFitAndResize, 600);
    };

    terminalWs.onmessage = (event) => {
        let data = event.data;
        if (data instanceof Blob) {
            const reader = new FileReader();
            reader.onload = () => term.write(reader.result);
            reader.readAsText(data);
        } else {
            term.write(data);
        }
    };

    terminalWs.onclose = (event) => {
        updateTerminalStatus('disconnected');
        term.writeln("\r\n\x1b[31m[系统] 连接已断开 (code: " + event.code + ")\x1b[0m");
        console.log('WebSocket closed:', event);
    };

    terminalWs.onerror = (error) => {
        updateTerminalStatus('error');
        term.writeln("\r\n\x1b[31m[系统] 连接错误，请检查网络或服务器\x1b[0m");
        console.error('WebSocket Error:', error);
    };

    // 监听容器大小变化，自动 fit
    const resizeObserver = new ResizeObserver(() => {
        if (fitAddon) {
            fitAddon.fit();
            // 可选：如果 WS 已连接，发送 resize
            if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                terminalWs.send(JSON.stringify({
                    type: "resize",
                    cols: term.cols,
                    rows: term.rows
                }));
            }
        }
    });
    resizeObserver.observe(document.getElementById('terminalContainer'));

    //venv处理
    // 读取 venvPath
    let venvPath = '';
    try {
        const context = JSON.parse(sessionStorage.getItem('websiteTerminalContext') || '{}');
        const projectPath = context.path || '';
        venvPath = context.venvPath || '';  
        console.log(venvPath)
        // 如果项目不在C盘，要换盘符到项目所在盘符
        // if (!projectPath.startsWith('C:')) {
        //     const changecmd = `chdir /d ${projectPath.split(':')[0]}:`;
        //     // terminalWs.send(changecmd + '\r\n');
        //     term.writeln(changecmd);
        //     console.log('自动切换盘符:', changecmd);
        // }
        if (projectPath) {
            setTimeout(() => {
                console.log('切换到项目目录:', projectPath);
                terminalWs.send(`cd ${projectPath}\r`);
                if (venvPath) {
                    const activateCmd = `${venvPath.replace(/\//g, '\\')}\\Scripts\\python.exe ${venvPath.replace(/\//g, '\\')}\\Scripts\\pip.exe`;
                    terminalWs.send(activateCmd);
                }
                sessionStorage.removeItem('websiteTerminalContext');
            }, 1500);  // 等待 shell 就绪
        }
    } catch (e) {
        console.warn('读取 venvPath 失败:', e);
    }

}

function updateTerminalStatus(status) {
    const statusEl = document.getElementById('terminalStatus');
    if (!statusEl) return;

    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('span:last-child');

    dot.className = 'status-dot';
    switch (status) {
        case 'connected':
            dot.classList.add('connected');
            text.textContent = '已连接';
            break;
        case 'connecting':
            text.textContent = '连接中...';
            break;
        case 'disconnected':
            text.textContent = '已断开';
            break;
        case 'error':
            dot.classList.add('error');
            text.textContent = '连接错误';
            break;
        default:
            text.textContent = '未知状态';
    }
}

document.addEventListener('DOMContentLoaded', () => {
    const connectBtn = document.getElementById('connectTerminalBtn');
    if (connectBtn) {
        connectBtn.addEventListener('click', () => {
            if (term) term.clear();
            connectTerminal();
        });
    }
    connectTerminal();  // 自动连接，可根据需要注释
});

window.connectTerminal = connectTerminal;