def define_fedora_vm(config, version)
  config.vm.define "fedora-#{version}" do |fedora|
    fedora.vm.box = "fedora/#{version}-cloud-base"
    fedora.vm.provision 'ansible' do |ansible|
      ansible.playbook = 'tests/playbooks/fedora.yaml'
    end
  end
end

Vagrant.configure('2') do |config|
  exclude = File.readlines('.gitignore').reject{|l| l.start_with? '#'}.map(&:strip).map(&Dir.method(:glob)).flatten(1)
  config.vm.synced_folder '.', '/vagrant', type: :rsync, rsync__exclude: exclude

  config.vm.provider :libvirt do |libvirt, override|
    libvirt.memory = 1024
    override.vm.synced_folder '.', '/vagrant', type: :nfs
    end

  define_fedora_vm config, '32'
  define_fedora_vm config, '33'
end
