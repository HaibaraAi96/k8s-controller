## We specify the base image we need for our
## go application
FROM golang:1.14-alpine
## We create a /controller directory within our
## image that will hold our application source
## files
RUN mkdir /controller
## We copy everything in the root directory
## into our /controller directory
ADD . /controller
## We specify that we now wish to execute 
## any further commands inside our /controller
## directory
WORKDIR /controller
## Add this go mod download command to pull in any dependencies
RUN go mod download
## we run go build to compile the binary
## executable of our Go program
RUN go build -o k8s-controller .
## Our start command which kicks off
## our newly created binary executable
CMD ["/controller/k8s-controller", "-kubeconfig=/controller/config.yaml"]