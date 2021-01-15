@Library('dst-shared@master') _

dockerBuildPipeline {
        githubPushRepo = "Cray-HPE/hms-reds"
        repository = "cray"
        imagePrefix = "cray"
        app = "reds"
        name = "hms-reds"
        description = "Cray river endpoint discovery service"
        dockerfile = "Dockerfile"
        slackNotification = ["", "", false, false, true, true]
        product = "csm"
}
