pipeline {
    agent any

    stages {
        stage('Lint') {
            steps {
                sh 'make lint'
            }
        }
        stage('Test') {
            steps {
                sh 'make test-integration'
            }
        }
        stage('Build Image') {
            steps {
                sh 'make build-image'
            }
        }
    }

    post {
        always {
            sh 'make clean'
        }
    }
}
