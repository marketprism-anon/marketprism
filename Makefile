DETERLAB=simulation/platform/ResearchLabab_users
SIMUL=simulation/manage/simulation
ORCHESTRATOR=orchestrator
CWD:=$(shell pwd)

all: build

create-builddirs:
	@mkdir -p build

build-deterlab:
	@echo "building users(executable) for oses/arch"
	make -C ${DETERLAB}
	@echo "Moving files to build folder"
	@mv ${DETERLAB}/users build/

build-simul:
	@echo "building simul(executable) for oses/arch"
	make -C ${SIMUL}
	@echo "Moving files to build folder"
	@mv ${SIMUL}/simul build/

build-orchestator:
	@echo "building orchestrator(executable) for oses/arch"
	make -C ${ORCHESTRATOR}
	@echo "Moving files to build folder"
	@mv ${ORCHESTRATOR}/orchestrator build/


build: clean create-builddirs build-deterlab build-simul copy-configs build-orchestator

build-deterlab-rc:
	@echo "building users(executable) for oses/arch"
	make -C ${DETERLAB} build-rc
	@echo "Moving files to build folder"
	@mv ${DETERLAB}/users build/

build-simul-rc:
	@echo "building simul(executable) for oses/arch"
	make -C ${SIMUL} build-rc
	@echo "Moving files to build folder"
	@mv ${SIMUL}/simul build/

build-orchestator-rc:
	@echo "building orchestrator(executable) for oses/arch"
	make -C ${ORCHESTRATOR}
	@echo "Moving files to build folder"
	@mv ${ORCHESTRATOR}/orchestrator build/


build-rc: clean create-builddirs build-deterlab-rc build-simul-rc copy-configs build-orchestator-rc


copy-configs:
	@echo "Copying Excel Files and Configs"
	@cp -r ${SIMUL}/deploy/*.* build/
	@cp -r ${ORCHESTRATOR}/ssh.toml build/

build-docker-rc:
	make -C race-cond-build-env
clean:
	@rm -rf build

deploy: all
	$(eval ARCH:=$(shell ssh ${USER}@ResearchLab-Url uname -m))
	$(eval OS:=$(shell ssh ${USER}@ResearchLab-Url uname -s))
	$(eval ARCH:=$(shell echo ${ARCH} | sed s/x86_64/amd64/))
	$(eval OS:=$(shell echo ${OS} | awk '{print tolower($0)}'))
	echo ${OS}
	echo ${ARCH}
	rsync -avz build/*.db ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/*.toml ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/*.bin ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/simul/${OS}/${ARCH}/simul ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/users/${OS}/${ARCH}/users ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/orchestrator/${OS}/${ARCH}/orchestrator ${USER}@ResearchLab-Url:~/remote

deploy-rc: build-docker-rc
	rsync -avz build/*.db ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/*.toml ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/*.bin ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/simul/linux/amd64/simul ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/users/linux/amd64/users ${USER}@ResearchLab-Url:~/remote
	rsync -avz build/orchestrator/linux/amd64/orchestrator ${USER}@ResearchLab-Url:~/remote

.PHONY: clean create-builddirs build
