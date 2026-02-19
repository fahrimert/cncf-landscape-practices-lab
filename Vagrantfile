Vagrant.configure("2") do |config|
  
  config.vm.box = "ubuntu/jammy64"
  config.vm.network "private_network", ip: "192.168.56.10"
  config.vm.disk :disk, size: "10GB", name: "ceph_osd_disk"
  config.vm.hostname = "mert-k3slab-server-cncf-landscape-practices"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "14336"   
    vb.cpus = 6         
    vb.name = "mert-k3slab-server-cncf-landscape-practices"
    
    vb.customize ["modifyvm", :id, "--ioapic", "on"]
    vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
    vb.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
    vb.customize ["modifyvm", :id, "--nested-hw-virt", "on"]
  end

end
