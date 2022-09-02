package launcher

import (
	"time"

	"EnclaveLauncher/connect"
	"EnclaveLauncher/instances"
	"EnclaveLauncher/keypairs"

	log "github.com/sirupsen/logrus"
)


func SetupInstance(profile string) (*connect.SshClient, *string, error){
	log.Info("Setting up instance!")
	keyPairName := "enclavetest"
	keyStoreLocation := "./enclavetest.pem"
	region := "ap-south-1"

	err := keypairs.SetupKeys(keyPairName, keyStoreLocation, profile, region)
	if err != nil {
		return nil, nil, err
	}
	newInstanceID, err := instances.LaunchInstance(keyPairName, profile, region)
	if err != nil {
		return nil, nil, err
	}
	time.Sleep(2 * time.Minute)
	instance, err := instances.GetInstanceDetails(*newInstanceID, profile, region)
	if err != nil {
		return nil, newInstanceID, err
	}
	client := connect.NewSshClient(
		"ubuntu",
		*(instance.PublicIpAddress),
		22,
		keyStoreLocation,
	)
	return client, newInstanceID, nil
}	


func RunEnclave(client *connect.SshClient) (string, error) {
	log.Info("Running Enclave!")
	return RunCommand(client, "nitro-cli run-enclave --cpu-count 2 --memory 4500 --eif-path startup.eif --debug-mode")
}

func TearDown(client *connect.SshClient, instanceID string, profile string) (error){
	log.Info("Tearing Down!")
	_, err := RunCommand(client, "nitro-cli terminate-enclave --all")
	if err != nil {
		return err
	}
	err = instances.TerminateInstance(instanceID, profile, "ap-south-1")
	return err
}

func SetupPreRequisites(client *connect.SshClient, host string, instanceID string, profile string, region string) {
	RunCommand(client, "sudo apt-get -y update")
	RunCommand(client, "sudo apt-get -y install sniproxy")
	RunCommand(client, "sudo service sniproxy start")
	RunCommand(client, "sudo apt-get -y install build-essential")
	RunCommand(client, "grep /boot/config-$(uname -r) -e NITRO_ENCLAVES")
	RunCommand(client, "sudo apt-get -y install linux-modules-extra-aws")
	RunCommand(client, "sudo apt-get -y install docker.io")
	RunCommand(client, "sudo systemctl start docker")
	RunCommand(client, "sudo systemctl enable docker")
	RunCommand(client, "sudo usermod -aG docker ubuntu")
	RunCommand(client, "git clone https://github.com/aws/aws-nitro-enclaves-cli.git")
	RunCommand(client, "cd aws-nitro-enclaves-cli && THIS_USER=\"$(whoami)\"")
	RunCommand(client, "cd aws-nitro-enclaves-cli && export NITRO_CLI_INSTALL_DIR=/")
	RunCommand(client, "cd aws-nitro-enclaves-cli && make nitro-cli")
	RunCommand(client, "cd aws-nitro-enclaves-cli && make vsock-proxy")
	RunCommand(client, `cd aws-nitro-enclaves-cli && 
						sudo make NITRO_CLI_INSTALL_DIR=/ install &&
						source /etc/profile.d/nitro-cli-env.sh && 
						echo source /etc/profile.d/nitro-cli-env.sh >> ~/.bashrc && 
						nitro-cli-config -i`)

	connect.TransferFile(client.Config, host, "./allocator.yaml", "allocator.yaml")

	
	RunCommand(client, "sudo systemctl start nitro-enclaves-allocator.service")
	RunCommand(client, "sudo cp allocator.yaml /etc/nitro_enclaves/allocator.yaml")
	instances.RebootInstance(instanceID, profile, region)
	time.Sleep(2 * time.Minute)
	RunCommand(client, "sudo systemctl start nitro-enclaves-allocator.service")
	RunCommand(client, "sudo systemctl enable nitro-enclaves-allocator.service")
}

func TransferAndLoadDockerImage(client *connect.SshClient, host string, file string, image string, destination string) {
	connect.TransferFile(client.Config, host, file, destination)

	RunCommand(client, "docker load < docker_image.tar")
}

func  RunCommand(client *connect.SshClient, cmd string) (string, error) {
	
	// fmt.Println("============================================================================================")
	// log.Info(cmd)
	// fmt.Println("")

	output, err := client.RunCommand(cmd)
	
	if err != nil {
		log.Error("SSH run command error %v", err)
		return "", err
	}
	return output, nil
}

