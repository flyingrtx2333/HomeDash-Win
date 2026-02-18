// terminal.js

import { Terminal } from 'xterm'; // 假设已安装 xterm
import { FitAddon } from 'xterm-addon-fit'; // 新增：引入 FitAddon

let terminalWs = null;
let term = null; // xterm 实例
let fitAddon = null; // FitAddon 实例
let currentLine = ''; // 用于本地回显的缓冲区（可选，xterm通常由后端回显）

function connectTerminal() {
    console.log('Connecting to terminal...');
    
    // 如果已经连接，先断开
    if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
        terminalWs.close();
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/terminal`;

    updateTerminalStatus('connecting');

    // 初始化 xterm.js
    if (!term) {
        term = new Terminal({
            cursorBlink: true,   // 光标闪烁
            fontSize: 14,        // 字体大小
            fontFamily: 'Consolas, "Courier New", monospace', // 字体
            theme: {
                background: '#1e1e1e', // 背景色
                foreground: '#d4d4d4', // 前景色
                cursor: '#ffffff',     // 光标颜色
                selection: 'rgba(255, 255, 255, 0.3)', // 选中背景
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
            }
        });

        // 新增：加载 FitAddon
        fitAddon = new FitAddon();
        term.loadAddon(fitAddon);

        // 将终端挂载到 DOM
        const container = document.getElementById('terminalContainer');
        term.open(container);
        
        // 自适应大小（使用 fitAddon）
        window.addEventListener('resize', () => {
            if (fitAddon) fitAddon.fit();
        });

        // 新增：监听 resize 事件并发送到后端
        term.onResize(({ cols, rows }) => {
            if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                terminalWs.send(JSON.stringify({
                    type: "resize",
                    cols: cols,
                    rows: rows
                }));
            }
        });
    }
    
    // 清空终端屏幕
    term.clear();

    // 建立 WebSocket 连接
    terminalWs = new WebSocket(wsUrl);

    terminalWs.onopen = () => {
        updateTerminalStatus('connected');
        term.writeln("\x1b[32m[系统] 已连接到远程终端...\x1b[0m");
        
        // 聚焦终端
        term.focus();
        
        // 新增：发送初始大小
        if (fitAddon) fitAddon.fit();
        terminalWs.send(JSON.stringify({
            type: "resize",
            cols: term.cols,
            rows: term.rows
        }));
        
        // 处理从浏览器发往服务器的数据 (用户输入)
        term.onData(data => {
            if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
                terminalWs.send(data);
            }
        });
    };

    terminalWs.onmessage = (event) => {
        // 接收服务器数据并写入终端
        // xterm.js 会自动处理 \n \r \b 以及 ANSI 颜色代码
        if (event.data instanceof Blob) {
            // 处理 Blob 数据 (某些服务器配置可能发送 Blob)
            const reader = new FileReader();
            reader.onload = function() {
                term.write(reader.result);
            };
            reader.readAsText(event.data);
        } else {
            term.write(event.data);
        }
    };

    terminalWs.onclose = (event) => {
        updateTerminalStatus('disconnected');
        term.writeln("\x1b[31m[系统] 连接已断开\x1b[0m");
    };

    terminalWs.onerror = (error) => {
        updateTerminalStatus('error');
        term.writeln("\x1b[31m[系统] 连接错误\x1b[0m");
        console.error('WebSocket Error:', error);
    };
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
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    // 绑定按钮事件
    const connectBtn = document.getElementById('connectTerminalBtn');
    if (connectBtn) {
        connectBtn.addEventListener('click', () => {
            if (term) term.clear(); // 重连时清屏
            connectTerminal();
        });
    }

    // 自动连接
    connectTerminal();
});