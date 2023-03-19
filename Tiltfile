docker_compose("./docker-compose.yaml")

dc_resource("nomad")
dc_resource("consul")

# Illustrates how you can run the Nomad command without installing it
local_resource(
    "Nomad version", 
    cmd="docker run --network host multani/nomad run --help",
    resource_deps=["nomad"]    
)

def nomad_job(
    name,
    jobspec,
    links = [],
    resource_deps = []
):
    if config.tilt_subcommand == 'down':
        print("tilt down")
        local("docker run --network host multani/nomad stop helloapp")

    if config.tilt_subcommand == 'up':
        # TODO: Add vars as needed
        # -var image_version=v0.2.0

        specfile = os.path.basename(jobspec)
        resource_deps.append("nomad")
        local_resource(
            name, 
            serve_cmd="docker run -v $(pwd)/" + jobspec + ":/app/" + specfile + " --network host multani/nomad run -verbose /app/" + specfile + " && go run main.go " + jobspec,
            resource_deps=resource_deps,
            links = links
        )

nomad_job("helloapp", "examples/hello.nomad")
nomad_job("batch", "examples/batch.nomad")