// Disable ESLint rules
/* eslint-disable no-console, global-require, no-undef, no-unused-vars */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { exec, fork } = require('child_process');
const net = require('net');
const seccomp = require('seccomp'); // Assuming a Node.js seccomp library exists
const express = require('express');
const bodyParser = require('body-parser');

const fileSockPath = "/host/comms.sock";
let fileSock = null;
let bootstrapPath = null;

function errExit(msg) {
    console.error(`${msg}: ${os.strerror(os.constants.errno.EINVAL)}`);
    process.exit(1);
}

function unshareNamespaces() {
    try {
        // Unshare UTS, PID, and IPC namespaces
        os.unshare(os.constants.CLONE_NEWUTS | os.constants.CLONE_NEWPID | os.constants.CLONE_NEWIPC);
    } catch (e) {
        errExit(`unshare failed: ${e}`);
    }
    return 0;
}

function enableSeccomp() {
    try {
        const data = JSON.parse(fs.readFileSync('syscalls.json', 'utf-8'));
        const calls = data.calls;

        const ctx = new seccomp.ScmpFilter(seccomp.ActErrno(1));

        calls.forEach(call => {
            try {
                ctx.addRule(seccomp.ActAllow, seccomp.ScmpSyscall(call));
            } catch (e) {
                return -1;
            }
        });

        ctx.load();
        return 0;
    } catch (e) {
        console.error(e);
        return -1;
    }
}

function forkProcess() {
    try {
        const res = fork();
        return res;
    } catch (e) {
        console.error(`Fork failed: ${e}`);
        process.exit(1);
    }
}

function webServer() {
    console.log(`server.js: start web server on fd: ${fileSock.fd}`);
    require.paths.push('/handler');

    const f = require('f');

    const app = express();
    app.use(bodyParser.json());

    app.post('/*', (req, res) => {
        try {
            const event = req.body;
            res.json(f.f(event));
        } catch (e) {
            res.status(500).send(e.stack);
        }
    });

    const server = app.listen({ path: fileSockPath }, () => {
        console.log('Server started');
    });
}

function forkServer() {
    fileSock.setBlocking(true);
    console.log(`server.js: start fork server on fd: ${fileSock.fd}`);

    fileSock.on('connection', (client) => {
        const buffer = Buffer.alloc(8);
        client.read(buffer, 0, 8, (err, bytesRead, buffer) => {
            if (err) throw err;
            const fds = buffer.readUInt32LE(0);
            const rootFd = fds[0];
            const memCgroupFd = fds[1];

            const pid = forkProcess();

            if (pid) {
                fs.closeSync(rootFd);
                fs.closeSync(memCgroupFd);

                process.wait(pid, 0);
                client.write(Buffer.from([pid]));
                client.end();
            } else {
                fileSock.close();
                fileSock = null;

                fs.fchdir(rootFd);
                fs.chroot(".");
                fs.closeSync(rootFd);

                fs.writeSync(memCgroupFd, Buffer.from(process.pid.toString()));
                fs.closeSync(memCgroupFd);

                startContainer();
                process.exit(1);
            }
        });
    });
}

function startContainer() {
    const returnVal = unshareNamespaces();
    if (returnVal !== 0) throw new Error('Unshare failed');

    fileSock = net.createServer().listen(fileSockPath);

    const pid = forkProcess();
    if (pid > 0) {
        process.exit(0);
    }

    fs.readFile(bootstrapPath, 'utf-8', (err, code) => {
        if (err) throw err;
        try {
            eval(code);
        } catch (e) {
            console.error(`Exception: ${e.stack}`);
            console.error(`Problematic JavaScript Code:\n${code}`);
        }
    });
}

function main() {
    if (process.argv.length < 3) {
        console.log("Expected execution: chroot <path_to_root_fs> node server.js <path_to_bootstrap.js> [cgroup-count] [enable-seccomp]");
        console.log("    cgroup-count: number of FDs (starting at 3) that refer to /sys/fs/cgroup/..../cgroup.procs files");
        console.log("    enable-seccomp: true/false to enable or disables seccomp filtering");
        process.exit(1);
    }

    console.log(`server.js: started new process with args: ${process.argv.join(' ')}`);

    if (process.argv.length < 4 || process.argv[3] === 'true') {
        const returnCode = enableSeccomp();
        if (returnCode < 0) throw new Error('Seccomp enabling failed');
        console.log('seccomp enabled');
    }

    bootstrapPath = process.argv[2];
    const cgroupFds = process.argv.length > 3 ? parseInt(process.argv[3], 10) : 0;

    const pid = process.pid.toString();
    for (let i = 0; i < cgroupFds; i++) {
        const fdId = 3 + i;
        fs.writeSync(fdId, pid);
        console.log(`server.js: joined cgroup, close FD ${fdId}`);
    }

    startContainer();
}

if (require.main === module) {
    main();
}