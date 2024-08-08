require 'socket'
require 'json'
require 'securerandom'
require 'fcntl'
require 'sys/proctable'
require 'seccomp-tools' # Assuming a Ruby seccomp library exists

file_sock_path = "/host/comms.sock"
file_sock = nil
bootstrap_path = nil

def err_exit(msg)
  STDERR.puts "#{msg}: #{Errno::EINVAL.message}"
  exit(1)
end

def unshare_namespaces
  begin
    # Unshare UTS, PID, and IPC namespaces
    Process.unshare(Process::CLONE_NEWUTS | Process::CLONE_NEWPID | Process::CLONE_NEWIPC)
  rescue SystemCallError => e
    err_exit("unshare failed: #{e}")
  end
  0
end

def enable_seccomp
  # Load the syscalls from the JSON file
  data = JSON.parse(File.read('syscalls.json'))
  calls = data['calls']

  # Initialize the seccomp filter
  ctx = SeccompTools::ScmpFilter.new(SeccompTools::ActErrno.new(1))

  # Add rules to allow the specified system calls
  calls.each do |call|
    begin
      ctx.add_rule(SeccompTools::ActAllow.new, SeccompTools::ScmpSyscall.new(call))
    rescue SeccompTools::SeccompError => e
      return -1
    end
  end

  # Load the filter into the kernel
  begin
    ctx.load
  rescue SeccompTools::SeccompError => e
    return -1
  end

  0
end

def fork_process
  begin
    res = fork
    return res
  rescue SystemCallError => e
    STDERR.puts "Fork failed: #{e}"
    exit(1)
  end
end

def web_server
  puts "server.rb: start web server on fd: #{file_sock.fileno}"
  $LOAD_PATH << '/handler'

  # TODO: as a safeguard, we should add a mechanism so that the
  # import doesn't happen until the cgroup move completes, so that a
  # malicious child cannot eat up Zygote resources
  require 'f'

  class SockFileHandler < Sinatra::Base
    post '/*' do
      begin
        data = request.body.read
        event = JSON.parse(data)
        content_type :json
        F.f(event).to_json
      rescue JSON::ParserError
        status 400
        "bad POST data: \"#{data}\""
      rescue => e
        status 500
        e.message
      end
    end
  end

  if defined?(F.app)
    # use WSGI entry
    app = F.app
  else
    # use function entry
    app = SockFileHandler
  end

  server = WEBrick::HTTPServer.new(Port: 8080, BindAddress: '127.0.0.1')
  server.mount '/', app
  trap 'INT' do server.shutdown end
  server.start
end

def fork_server
  file_sock.fcntl(Fcntl::F_SETFL, Fcntl::O_NONBLOCK)
  puts "server.rb: start fork server on fd: #{file_sock.fileno}"

  loop do
    client, _info = file_sock.accept
    _, fds, _, _ = Socket.recv_io(client, 8, 2)
    root_fd, mem_cgroup_fd = fds

    pid = fork_process

    if pid
      # parent
      IO.new(root_fd).close
      IO.new(mem_cgroup_fd).close

      # the child opens the new ol.sock, forks the grandchild
      # (which will actually do the serving), then exits.  Thus,
      # by waiting for the child, we can be sure ol.sock exists
      # before we respond to the client that sent us the fork
      # request with the root FD.  This means the client doesn't
      # need to poll for ol.sock existence, because it is
      # guaranteed to exist.
      Process.wait(pid)
      client.send([pid].pack("I"))
      client.close
    else
      # child
      file_sock.close
      file_sock = nil

      # chroot
      Dir.chdir(IO.new(root_fd))
      Dir.chroot(".")
      IO.new(root_fd).close

      # mem cgroup
      IO.new(mem_cgroup_fd).write(Process.pid.to_s)
      IO.new(mem_cgroup_fd).close

      # child
      start_container
      exit(1) # only reachable if program unexpectedly returns
    end
  end
end

def start_container
  '''
  1. this assumes chroot has taken us to the location where the
      container should start.
  2. it launches the container code by running whatever is in the
      bootstrap file (from argv)
  '''

  return_val = unshare_namespaces
  raise 'unshare failed' unless return_val == 0

  file_sock = UNIXServer.new(file_sock_path)

  pid = fork_process
  raise 'fork failed' unless pid >= 0

  if pid > 0
    # orphan the new process by exiting parent.  The parent
    # process is in a weird state because unshare only partially
    # works for the process that calls it.
    exit(0)
  end

  File.open(bootstrap_path, 'r') do |f|
    code = f.read
    begin
      eval(code)
    rescue => e
      puts "Exception: #{e.message}"
      puts "Problematic Ruby Code:\n#{code}"
    end
  end
end

def main
  '''
  caller is expected to do chroot, because we want to use the
  ruby executable inside the container
  '''

  if ARGV.length < 2
    puts "Expected execution: chroot <path_to_root_fs> ruby server.rb <path_to_bootstrap.rb> [cgroup-count] [enable-seccomp]"
    puts "    cgroup-count: number of FDs (starting at 3) that refer to /sys/fs/cgroup/..../cgroup.procs files"
    puts "    enable-seccomp: true/false to enable or disables seccomp filtering"
    exit(1)
  end

  puts "server.rb: started new process with args: #{ARGV.join(' ')}"

  # enable_seccomp if enable-seccomp is not passed
  if ARGV.length < 3 || ARGV[3] == 'true'
    return_code = enable_seccomp
    raise 'seccomp enabling failed' unless return_code >= 0
    puts 'seccomp enabled'
  end

  bootstrap_path = ARGV[1]
  cgroup_fds = ARGV.length > 2 ? ARGV[2].to_i : 0

  # join cgroups passed to us.  The fact that chroot is called
  # before we start means we also need to pass FDs to the cgroups we
  # want to join, because chroot happens before we run, so we can no
  # longer reach them by paths.
  pid = Process.pid.to_s
  cgroup_fds.times do |i|
    fd_id = 3 + i
    IO.new(fd_id, 'w') do |file|
      file.write(pid)
      puts "server.rb: joined cgroup, close FD #{fd_id}"
    end
  end

  start_container
end

if __FILE__ == $0
  main
end