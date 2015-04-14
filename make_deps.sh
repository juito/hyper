#!/bin/sh

gopath=$GOPATH

if [ ! -e $gopath ]
then
  echo "Please make sure you have GOPATH in environment!\n"
  exit 1
fi

git clone https://github.com/gorilla/context $gopath/src/github.com/gorilla/context
git clone https://github.com/gorilla/mux $gopath/src/github.com/gorilla/mux
