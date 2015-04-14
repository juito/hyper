#!/bin/sh

rootpath=$GOPATH
echo $rootpath
if [ ! -e "$rootpath" ]
then
  echo "Please make sure you have GOPATH environment!\n"
  exit 1
fi

deps="./deps/*"
cp -r $deps $rootpath/src
