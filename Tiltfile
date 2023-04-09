docker_compose("./docker-compose.yaml")

dc_resource("nomad")
dc_resource("consul")

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
        
        final_resource_deps = ["nomad"]
        final_resource_deps.extend(resource_deps)

        final_links = [link("http://localhost:4646/ui/jobs/" + name, "Nomad UI for Job")]
        final_links.extend(links)
        local_resource(
            name, 
            serve_cmd="docker run -v $(pwd)/" + jobspec + ":/app/" + specfile + " --network host multani/nomad run -verbose /app/" + specfile + " && go run . " + jobspec,
            resource_deps = final_resource_deps,
            links = final_links
        )

nomad_job("helloapp", "examples/hello/hello.nomad")
nomad_job("batch", "examples/batch/batch.nomad")