### 0Chain REST API Documentation
-------------------------
   Documentation is via Postman docs. It is hosted at https://api.0chain.net  
   Deprecated documentation was via redoc/swagger and can be found in the redoc folder.

#### Editing API documentation  
Open the json postman collection in postman.  
Edit the collection as needed.  
Open a PR and commit to master. 
This will automatically sync with postman and render at the public url above.


mvn install:install-file -Dfile=/Users/ryanstewart/Downloads/wasmer-jni-amd64-darwin-0.3.0.jar  -DgroupId=org.wasmer -DartifactId=wasmer-darwin-Dversion=1.0.0 -Dpackaging=jar


1. Build, package, publish, static code analysis (what we have today)
(Additionally Publish code smells and code coverage info to PR and block it from merging if criteria not met)
2. Auto deploy new code image as well as full blockchain develop images to Kubernetes
3. Run postman api regressions against Kubernetes instance
4. Run conductor/load tests
5. Run CLI integration tests

Additionally on develop:
6. Deploy to devnet
7. Run api tests against devnet
8. Run cli tests against devnet 