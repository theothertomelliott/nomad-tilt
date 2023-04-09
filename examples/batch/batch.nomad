job "batch" {
  datacenters = ["dc1"]

  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value = "linux"
  }

  periodic {
    // Launch every 5 seconds
    cron = "*/5 * * * * * *"

    // Do not allow overlapping runs.
    prohibit_overlap = true
  }

  group "batch" {
    count = 1

    restart {
      interval = "20s"
      attempts = 2
      delay    = "5s"
      mode     = "delay"
    }

    task "date" {
      driver = "raw_exec"

      service {
        name = "date-batch-job"
        tags = ["date"]
        port = "date"

        check {
          name     = "alive"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }
      }

      config {
        command = "date"
      }

      resources {
        cpu = 100 # Mhz
        memory = 128 # MB

        network {
          mbits = 1

          # Request for a dynamic port
          port "date" {
          }
        }
      }
    }
  }
}